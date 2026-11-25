//go:build integration

package persistent_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/outbox"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/pgtest"
)

var errRollback = errors.New("the change that caused the email failed")

type outboxEnv struct {
	tx     repo.Transactor
	outbox repo.OutboxRepo
}

func (e outboxEnv) do(ctx context.Context, t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, e.tx.InScope(ctx, repo.Scope{}, fn))
}

func (e outboxEnv) exec(ctx context.Context, t *testing.T, sql string, args ...any) {
	t.Helper()
	e.do(ctx, t, func(ctx context.Context) error {
		tx, err := persistent.Tx(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, sql, args...)
		return err
	})
}

func (e outboxEnv) enqueue(ctx context.Context, t *testing.T, n int) []string {
	t.Helper()
	ids := make([]string, n)
	e.do(ctx, t, func(ctx context.Context) error {
		for i := range ids {
			ids[i] = newID()
			if err := e.outbox.Enqueue(ctx, entity.Email{
				ID: ids[i], Kind: entity.EmailPasswordReset, To: fmt.Sprintf("user%d@acme.test", i),
				Data: entity.EmailData{FullName: "Alice", Token: "raw-token-" + ids[i]},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return ids
}

func (e outboxEnv) claim(ctx context.Context, t *testing.T) []entity.Email {
	t.Helper()
	var emails []entity.Email
	e.do(ctx, t, func(ctx context.Context) error {
		var err error
		emails, err = e.outbox.ClaimDue(ctx)
		return err
	})
	return emails
}

type outboxRow struct {
	status       string
	attempts     int
	hasPayload   bool
	sent         bool
	lastError    *string
	dueInSeconds float64
}

func (e outboxEnv) row(ctx context.Context, t *testing.T, id string) outboxRow {
	t.Helper()
	var r outboxRow
	e.do(ctx, t, func(ctx context.Context) error {
		tx, err := persistent.Tx(ctx)
		require.NoError(t, err)
		return tx.QueryRow(ctx, `SELECT status, attempts, payload IS NOT NULL, sent_at IS NOT NULL, last_error,
			extract(epoch FROM next_attempt_at - clock_timestamp())::float8 FROM outbox_emails WHERE id = $1`, id).
			Scan(&r.status, &r.attempts, &r.hasPayload, &r.sent, &r.lastError, &r.dueInSeconds)
	})
	return r
}

func ids(emails []entity.Email) []string {
	out := make([]string, len(emails))
	for i, e := range emails {
		out[i] = e.ID
	}
	return out
}

//nolint:paralleltest // the subtests share one database and empty the outbox first
func TestOutbox(t *testing.T) {
	db := pgtest.Start(t)
	ctx := t.Context()
	require.NoError(t, persistent.Migrate(ctx, db.OwnerURL, slog.New(slog.DiscardHandler)))
	pool := pgtest.Pool(ctx, t, db.AppURL, 8)
	env := outboxEnv{tx: persistent.NewTransactor(pool), outbox: outbox.New()}
	reset := func(t *testing.T) { t.Helper(); env.exec(ctx, t, "DELETE FROM outbox_emails") }

	t.Run("enqueue rolls back with its unit of work", func(t *testing.T) {
		reset(t)
		err := env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			require.NoError(t, env.outbox.Enqueue(ctx, entity.Email{ID: newID(), Kind: entity.EmailPasswordChanged, To: "a@acme.test"}))
			return errRollback
		})
		require.ErrorIs(t, err, errRollback)
		assert.Empty(t, env.claim(ctx, t))
	})

	t.Run("enqueue outside a unit of work is refused", func(t *testing.T) {
		err := env.outbox.Enqueue(ctx, entity.Email{ID: newID(), Kind: entity.EmailPasswordChanged, To: "a@acme.test"})
		require.ErrorIs(t, err, persistent.ErrNoUnitOfWork)
	})

	t.Run("claim returns the email with its data and counts the attempt", func(t *testing.T) {
		reset(t)
		id := env.enqueue(ctx, t, 1)[0]

		emails := env.claim(ctx, t)
		require.Len(t, emails, 1)
		assert.Equal(t, entity.Email{
			ID: id, Kind: entity.EmailPasswordReset, To: "user0@acme.test", Attempts: 1,
			Data: entity.EmailData{FullName: "Alice", Token: "raw-token-" + id},
		}, emails[0])

		r := env.row(ctx, t, id)
		assert.Equal(t, "pending", r.status)
		assert.InDelta(t, 5*60, r.dueInSeconds, 5, "leased for 5 minutes")
		assert.Empty(t, env.claim(ctx, t), "a leased email is not claimed again")
	})

	t.Run("claims at most 10 rows, oldest first", func(t *testing.T) {
		reset(t)
		all := env.enqueue(ctx, t, 12)
		env.exec(ctx, t, "UPDATE outbox_emails SET next_attempt_at = now() - interval '1 hour' WHERE id = ANY($1::uuid[])", all[:2])
		first := env.claim(ctx, t)
		require.Len(t, first, 10)
		assert.Subset(t, ids(first), all[:2])
		assert.Len(t, env.claim(ctx, t), 2)
	})

	t.Run("two concurrent claimers never get the same row", func(t *testing.T) {
		reset(t)
		env.enqueue(ctx, t, 15)

		// A claims and keeps its transaction open; B claims meanwhile and skips A's locked rows.
		claimed, release := make(chan []entity.Email), make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(t, env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
				emails, err := env.outbox.ClaimDue(ctx)
				claimed <- emails
				<-release
				return err
			}))
		})
		a := <-claimed
		b := env.claim(ctx, t)
		close(release)
		wg.Wait()

		require.Len(t, a, 10)
		require.Len(t, b, 5)
		for _, e := range b {
			assert.NotContains(t, ids(a), e.ID)
		}
	})

	t.Run("many concurrent claimers each get distinct rows", func(t *testing.T) {
		reset(t)
		want := env.enqueue(ctx, t, 60)

		var mu sync.Mutex
		var got []string
		var wg sync.WaitGroup
		for range 6 {
			wg.Go(func() {
				for {
					var emails []entity.Email
					err := env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
						var err error
						emails, err = env.outbox.ClaimDue(ctx)
						return err
					})
					if !assert.NoError(t, err) || len(emails) == 0 {
						return
					}
					mu.Lock()
					got = append(got, ids(emails)...)
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		assert.ElementsMatch(t, want, got, "every row claimed exactly once")
	})

	t.Run("backoff 1, 2, 4… minutes, failed after 8 attempts", func(t *testing.T) {
		reset(t)
		id := env.enqueue(ctx, t, 1)[0]

		for attempt := 1; attempt <= 8; attempt++ {
			env.exec(ctx, t, "UPDATE outbox_emails SET next_attempt_at = now() WHERE id = $1", id)
			emails := env.claim(ctx, t)
			require.Len(t, emails, 1, "attempt %d", attempt)
			require.Equal(t, attempt, emails[0].Attempts)
			env.do(ctx, t, func(ctx context.Context) error {
				return env.outbox.Reschedule(ctx, id, fmt.Sprintf("smtp failure %d", attempt))
			})

			r := env.row(ctx, t, id)
			require.Equal(t, attempt, r.attempts)
			require.Equal(t, "smtp failure "+fmt.Sprint(attempt), *r.lastError)
			if attempt < 8 {
				require.Equal(t, "pending", r.status, "attempt %d", attempt)
				assert.InDelta(t, 60*math.Pow(2, float64(attempt-1)), r.dueInSeconds, 5, "attempt %d", attempt)
			} else {
				require.Equal(t, "failed", r.status)
				assert.False(t, r.hasPayload, "failing clears the payload")
			}
		}

		env.exec(ctx, t, "UPDATE outbox_emails SET next_attempt_at = now() WHERE id = $1", id)
		assert.Empty(t, env.claim(ctx, t), "a failed email is not claimed")
	})

	t.Run("a crash during the last attempt fails the email at the next claim", func(t *testing.T) {
		reset(t)
		id := env.enqueue(ctx, t, 1)[0]
		// The 8th attempt was claimed, then the process died: the lease expired without an outcome.
		env.exec(ctx, t, "UPDATE outbox_emails SET attempts = 8, next_attempt_at = now() WHERE id = $1", id)

		assert.Empty(t, env.claim(ctx, t))
		r := env.row(ctx, t, id)
		assert.Equal(t, "failed", r.status)
		assert.Equal(t, 8, r.attempts, "no 9th attempt")
		assert.False(t, r.hasPayload)
		require.NotNil(t, r.lastError)
	})

	t.Run("a password reset older than its token is not sent", func(t *testing.T) {
		reset(t)
		reset1h := env.enqueue(ctx, t, 1)[0]
		changed := newID()
		env.do(ctx, t, func(ctx context.Context) error {
			return env.outbox.Enqueue(ctx, entity.Email{ID: changed, Kind: entity.EmailPasswordChanged, To: "a@acme.test"})
		})
		env.exec(ctx, t, "UPDATE outbox_emails SET created_at = now() - interval '61 minutes'")

		emails := env.claim(ctx, t)
		assert.Equal(t, []string{changed}, ids(emails), "only the password reset expires")
		r := env.row(ctx, t, reset1h)
		assert.Equal(t, "failed", r.status)
		assert.False(t, r.hasPayload, "the expired token is cleared")
	})

	t.Run("release undoes the attempt and makes the email due now", func(t *testing.T) {
		reset(t)
		all := env.enqueue(ctx, t, 2)
		require.Len(t, env.claim(ctx, t), 2)
		env.do(ctx, t, func(ctx context.Context) error { return env.outbox.Release(ctx, all[1:]) })

		emails := env.claim(ctx, t)
		require.Len(t, emails, 1)
		assert.Equal(t, all[1], emails[0].ID)
		assert.Equal(t, 1, emails[0].Attempts, "the released claim did not count")
	})

	t.Run("sent clears the payload and the last error", func(t *testing.T) {
		reset(t)
		id := env.enqueue(ctx, t, 1)[0]
		env.claim(ctx, t)
		env.do(ctx, t, func(ctx context.Context) error { return env.outbox.Reschedule(ctx, id, "smtp failure") })
		env.exec(ctx, t, "UPDATE outbox_emails SET next_attempt_at = now() WHERE id = $1", id)
		env.claim(ctx, t)
		env.do(ctx, t, func(ctx context.Context) error { return env.outbox.MarkSent(ctx, id) })

		r := env.row(ctx, t, id)
		assert.Equal(t, "sent", r.status)
		assert.True(t, r.sent)
		assert.False(t, r.hasPayload, "the token does not stay in the database")
		assert.Nil(t, r.lastError)
		assert.Empty(t, env.claim(ctx, t))
	})

	t.Run("last_error is bounded by the table", func(t *testing.T) {
		reset(t)
		id := env.enqueue(ctx, t, 1)[0]
		env.claim(ctx, t)
		err := env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			return env.outbox.Reschedule(ctx, id, strings.Repeat("x", 1001))
		})
		require.Error(t, err)
	})

	t.Run("maintenance clears failed payloads and purges sent rows by age", func(t *testing.T) {
		reset(t)
		rows := env.enqueue(ctx, t, 4)
		failed, failedRecent, sent, pending := rows[0], rows[1], rows[2], rows[3]
		env.exec(ctx, t, "UPDATE outbox_emails SET status = 'failed', created_at = now() - interval '2 hours' WHERE id = $1", failed)
		env.exec(ctx, t, "UPDATE outbox_emails SET status = 'failed' WHERE id = $1", failedRecent)
		env.exec(ctx, t, "UPDATE outbox_emails SET status = 'sent', sent_at = now() - interval '31 days', payload = NULL WHERE id = $1", sent)

		now := time.Now()
		env.do(ctx, t, func(ctx context.Context) error {
			cleared, err := env.outbox.ClearFailedPayloads(ctx, now.Add(-time.Hour))
			require.NoError(t, err)
			assert.Equal(t, int64(1), cleared)
			purged, err := env.outbox.PurgeSent(ctx, now.Add(-30*24*time.Hour))
			require.NoError(t, err)
			assert.Equal(t, int64(1), purged)
			return nil
		})

		assert.False(t, env.row(ctx, t, failed).hasPayload)
		assert.True(t, env.row(ctx, t, failedRecent).hasPayload, "a token younger than 1 hour is kept")
		assert.True(t, env.row(ctx, t, pending).hasPayload)
		env.do(ctx, t, func(ctx context.Context) error {
			assert.Equal(t, 3, count(ctx, t, "SELECT count(*) FROM outbox_emails"))
			return nil
		})
	})
}

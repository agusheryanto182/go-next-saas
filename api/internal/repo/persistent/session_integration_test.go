//go:build integration

package persistent_test

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/pgtest"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/session"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/user"
)

type sessionEnv struct {
	tx       repo.Transactor
	sessions repo.SessionRepo
	users    repo.UserRepo
}

func (e sessionEnv) do(ctx context.Context, t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, e.tx.InScope(ctx, repo.Scope{}, fn))
}

func (e sessionEnv) newUser(ctx context.Context, t *testing.T) string {
	t.Helper()
	id := newID()
	e.do(ctx, t, func(ctx context.Context) error {
		return e.users.Store(ctx, &entity.User{ID: id, Email: id + "@sessions.test", FullName: "Session User"})
	})
	return id
}

// store creates a session for userID that started at start and returns it with its token hash.
func (e sessionEnv) store(ctx context.Context, t *testing.T, userID string, start time.Time) (entity.Session, []byte) {
	t.Helper()
	s := entity.NewSession(newID(), userID, start, "203.0.113.7", "Firefox")
	hash := sha256.Sum256([]byte(s.ID))
	e.do(ctx, t, func(ctx context.Context) error { return e.sessions.Store(ctx, s, hash[:]) })
	return s, hash[:]
}

func (e sessionEnv) lookup(ctx context.Context, t *testing.T, hash []byte, now time.Time) (entity.Session, entity.User, error) {
	t.Helper()
	var (
		s   entity.Session
		u   entity.User
		err error
	)
	e.do(ctx, t, func(ctx context.Context) error {
		s, u, err = e.sessions.GetActiveByTokenHash(ctx, hash, now)
		return nil
	})
	return s, u, err
}

func (e sessionEnv) touch(ctx context.Context, t *testing.T, id string, now time.Time) bool {
	t.Helper()
	var touched bool
	e.do(ctx, t, func(ctx context.Context) error {
		var err error
		touched, err = e.sessions.Touch(ctx, id, now)
		return err
	})
	return touched
}

func (e sessionEnv) exists(ctx context.Context, t *testing.T, id string) bool {
	t.Helper()
	var n int
	e.do(ctx, t, func(ctx context.Context) error {
		tx, err := persistent.Tx(ctx)
		require.NoError(t, err)
		return tx.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE id = $1", id).Scan(&n)
	})
	return n == 1
}

//nolint:paralleltest // the subtests share one database
func TestSessions(t *testing.T) {
	db := pgtest.Start(t)
	ctx := t.Context()
	require.NoError(t, persistent.Migrate(ctx, db.OwnerURL, slog.New(slog.DiscardHandler)))
	env := sessionEnv{tx: persistent.NewTransactor(pgtest.Pool(ctx, t, db.AppURL, 4)), sessions: session.New(), users: user.New()}
	// PostgreSQL keeps microseconds and pgx returns local time; the clock is the use case's, not the database's.
	start := time.Now().Truncate(time.Microsecond)
	day := 24 * time.Hour

	t.Run("lookup returns the session with its user, without the password hash", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		want, hash := env.store(ctx, t, userID, start)

		s, u, err := env.lookup(ctx, t, hash, start.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, want, s)
		assert.Equal(t, userID, u.ID)
		assert.Equal(t, "Session User", u.FullName)
		assert.Empty(t, u.PasswordHash)
	})

	t.Run("lookup refuses unknown, revoked, idle-expired and absolute-expired sessions", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		_, idleHash := env.store(ctx, t, userID, start)
		revoked, revokedHash := env.store(ctx, t, userID, start)
		env.do(ctx, t, func(ctx context.Context) error {
			return env.sessions.Revoke(ctx, userID, revoked.ID, entity.RevokeLogout, start.Add(time.Minute))
		})
		busy, busyHash := env.store(ctx, t, userID, start)
		// Used every day: the idle expiry has caught up with the absolute one.
		for d := 1; d < 30; d++ {
			require.True(t, env.touch(ctx, t, busy.ID, start.Add(time.Duration(d)*day)))
		}

		unknown := sha256.Sum256([]byte("never issued"))
		for name, tc := range map[string]struct {
			hash []byte
			at   time.Time
		}{
			"unknown":            {unknown[:], start.Add(time.Minute)},
			"revoked":            {revokedHash, start.Add(2 * time.Minute)},
			"idle for 7 days":    {idleHash, start.Add(7 * day)},
			"after 30 days":      {busyHash, start.Add(30 * day)},
			"well after 30 days": {busyHash, start.Add(31 * day)},
		} {
			_, _, err := env.lookup(ctx, t, tc.hash, tc.at)
			assert.ErrorIs(t, err, entity.ErrSessionNotFound, name)
		}

		_, _, err := env.lookup(ctx, t, idleHash, start.Add(7*day-time.Second))
		require.NoError(t, err, "just before the idle expiry")
		_, _, err = env.lookup(ctx, t, busyHash, start.Add(30*day-time.Second))
		require.NoError(t, err, "a busy session lives until its absolute expiry")
	})

	t.Run("touch writes at most once an hour and extends the idle expiry", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		s, hash := env.store(ctx, t, userID, start)

		assert.False(t, env.touch(ctx, t, s.ID, start.Add(59*time.Minute)), "within the hour")
		assert.False(t, env.touch(ctx, t, s.ID, start.Add(time.Hour)), "exactly an hour")
		later := start.Add(2 * time.Hour)
		assert.True(t, env.touch(ctx, t, s.ID, later))
		assert.False(t, env.touch(ctx, t, s.ID, later.Add(time.Minute)), "a concurrent request right after writes nothing")

		got, _, err := env.lookup(ctx, t, hash, later.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, later, got.LastSeenAt)
		assert.Equal(t, later.Add(7*day), got.IdleExpiresAt)
		assert.Equal(t, s.AbsoluteExpiresAt, got.AbsoluteExpiresAt, "the absolute expiry never moves")
	})

	t.Run("touch caps the idle expiry at the absolute expiry", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		s, hash := env.store(ctx, t, userID, start)
		at := start.Add(25 * day)
		// Keep it alive up to day 25, then refresh: now + 7 days would pass day 30.
		for d := 1; d <= 25; d++ {
			require.True(t, env.touch(ctx, t, s.ID, start.Add(time.Duration(d)*day)))
		}

		got, _, err := env.lookup(ctx, t, hash, at.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, s.AbsoluteExpiresAt, got.IdleExpiresAt)
	})

	t.Run("touch leaves revoked and expired sessions alone", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		s, _ := env.store(ctx, t, userID, start)
		env.do(ctx, t, func(ctx context.Context) error {
			return env.sessions.Revoke(ctx, userID, s.ID, entity.RevokeLogout, start.Add(time.Minute))
		})
		assert.False(t, env.touch(ctx, t, s.ID, start.Add(2*time.Hour)), "revoked")

		idle, idleHash := env.store(ctx, t, userID, start)
		assert.False(t, env.touch(ctx, t, idle.ID, start.Add(7*day)), "idle-expired: not revived")
		_, _, err := env.lookup(ctx, t, idleHash, start.Add(7*day))
		require.ErrorIs(t, err, entity.ErrSessionNotFound)
	})

	t.Run("revoke ends only an active session of that user", func(t *testing.T) {
		alice, mallory := env.newUser(ctx, t), env.newUser(ctx, t)
		s, hash := env.store(ctx, t, alice, start)
		at := start.Add(time.Minute)

		err := env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			return env.sessions.Revoke(ctx, mallory, s.ID, entity.RevokeSignedOutElsewhere, at)
		})
		require.ErrorIs(t, err, entity.ErrSessionNotFound, "another user's session")
		_, _, err = env.lookup(ctx, t, hash, at)
		require.NoError(t, err, "still active")

		env.do(ctx, t, func(ctx context.Context) error {
			return env.sessions.Revoke(ctx, alice, s.ID, entity.RevokeLogout, at)
		})
		err = env.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			return env.sessions.Revoke(ctx, alice, s.ID, entity.RevokeLogout, at.Add(time.Minute))
		})
		require.ErrorIs(t, err, entity.ErrSessionNotFound, "already revoked")
	})

	t.Run("revoke all except one", func(t *testing.T) {
		alice, bob := env.newUser(ctx, t), env.newUser(ctx, t)
		current, currentHash := env.store(ctx, t, alice, start)
		_, laptopHash := env.store(ctx, t, alice, start)
		_, phoneHash := env.store(ctx, t, alice, start)
		_, bobHash := env.store(ctx, t, bob, start)
		at := start.Add(time.Minute)

		var n int64
		env.do(ctx, t, func(ctx context.Context) error {
			var err error
			n, err = env.sessions.RevokeOthers(ctx, alice, current.ID, entity.RevokePasswordChanged, at)
			return err
		})
		assert.Equal(t, int64(2), n)
		for name, hash := range map[string][]byte{"laptop": laptopHash, "phone": phoneHash} {
			_, _, err := env.lookup(ctx, t, hash, at)
			assert.ErrorIs(t, err, entity.ErrSessionNotFound, name)
		}
		_, _, err := env.lookup(ctx, t, currentHash, at)
		require.NoError(t, err, "the current session stays")
		_, _, err = env.lookup(ctx, t, bobHash, at)
		require.NoError(t, err, "other users are untouched")
	})

	t.Run("purge deletes sessions that ended before the cutoff", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		now := start.Add(200 * day)
		cutoff := now.Add(-90 * day)
		revoke := func(s entity.Session, at time.Time) {
			env.do(ctx, t, func(ctx context.Context) error {
				return env.sessions.Revoke(ctx, userID, s.ID, entity.RevokeLogout, at)
			})
		}

		idleBefore, _ := env.store(ctx, t, userID, cutoff.Add(-8*day)) // idle expiry a day before the cutoff
		idleAfter, _ := env.store(ctx, t, userID, cutoff.Add(-6*day))  // idle expiry a day after it
		// Both would idle-expire after the cutoff; a revocation ends a session when it happens.
		revokedBefore, _ := env.store(ctx, t, userID, cutoff.Add(-3*day))
		revoke(revokedBefore, cutoff.Add(-2*day))
		revokedAfter, _ := env.store(ctx, t, userID, cutoff.Add(-3*day))
		revoke(revokedAfter, cutoff.Add(day))
		active, _ := env.store(ctx, t, userID, now)

		var n int64
		env.do(ctx, t, func(ctx context.Context) error {
			var err error
			n, err = env.sessions.PurgeEnded(ctx, cutoff)
			return err
		})
		assert.GreaterOrEqual(t, n, int64(2), "plus the sessions of earlier subtests, all ended by now")
		assert.False(t, env.exists(ctx, t, idleBefore.ID))
		assert.False(t, env.exists(ctx, t, revokedBefore.ID))
		assert.True(t, env.exists(ctx, t, idleAfter.ID))
		assert.True(t, env.exists(ctx, t, revokedAfter.ID))
		assert.True(t, env.exists(ctx, t, active.ID))
	})

	t.Run("an unknown IP and user agent are stored as NULL", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		s := entity.NewSession(newID(), userID, start, "", "")
		hash := sha256.Sum256([]byte(s.ID))
		env.do(ctx, t, func(ctx context.Context) error { return env.sessions.Store(ctx, s, hash[:]) })

		got, _, err := env.lookup(ctx, t, hash[:], start)
		require.NoError(t, err)
		assert.Empty(t, got.IP)
		assert.Empty(t, got.UserAgent)
	})

	t.Run("the table refuses what the domain forbids", func(t *testing.T) {
		userID := env.newUser(ctx, t)
		s, hash := env.store(ctx, t, userID, start)
		dup := entity.NewSession(newID(), userID, start, "", "")
		for name, fn := range map[string]func(ctx context.Context) error{
			"a token hash used twice": func(ctx context.Context) error { return env.sessions.Store(ctx, dup, hash) },
			"a token hash of the wrong length": func(ctx context.Context) error {
				return env.sessions.Store(ctx, entity.NewSession(newID(), userID, start, "", ""), []byte("short"))
			},
			"an unknown revocation reason": func(ctx context.Context) error {
				return env.sessions.Revoke(ctx, userID, s.ID, entity.RevokeReason("bored"), start)
			},
			"a user agent above 512 characters": func(ctx context.Context) error {
				long := entity.NewSession(newID(), userID, start, "", strings.Repeat("a", 513))
				h := sha256.Sum256([]byte(long.ID))
				return env.sessions.Store(ctx, long, h[:])
			},
		} {
			assert.Error(t, env.tx.InScope(ctx, repo.Scope{}, fn), name)
		}
	})

	t.Run("outside a unit of work the repository is refused", func(t *testing.T) {
		_, _, err := env.sessions.GetActiveByTokenHash(ctx, make([]byte, 32), start)
		require.ErrorIs(t, err, persistent.ErrNoUnitOfWork)
	})
}

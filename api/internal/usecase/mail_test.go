package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase/mail"
)

var (
	errSMTP = errors.New("smtp: 451 try again later")
	errDB   = errors.New("database unavailable")
)

type mailDeps struct {
	tx     *MockTransactor
	outbox *MockOutboxRepo
	mailer *MockMailer
}

func newMail(t *testing.T) (usecase.Mail, mailDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := mailDeps{tx: NewMockTransactor(ctrl), outbox: NewMockOutboxRepo(ctrl), mailer: NewMockMailer(ctrl)}
	// Every unit of work in the mail use case has an empty scope: the outbox is global.
	d.tx.EXPECT().InScope(gomock.Any(), repo.Scope{}, gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, _ repo.Scope, fn func(context.Context) error) error { return fn(ctx) })
	return mail.New(d.tx, d.outbox, d.mailer, slog.New(slog.DiscardHandler)), d
}

func resetEmail(id string) entity.Email {
	return entity.Email{
		ID: id, Kind: entity.EmailPasswordReset, To: "alice@acme.test", Attempts: 1,
		Data: entity.EmailData{FullName: "Alice Liddell", Token: "s3cret-reset-token"},
	}
}

func TestMailSendDue(t *testing.T) {
	t.Parallel()

	t.Run("nothing due", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return([]entity.Email{}, nil)

		n, err := uc.SendDue(t.Context())
		require.NoError(t, err)
		assert.Zero(t, n)
	})

	t.Run("sends every claimed email and marks it sent", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		emails := []entity.Email{resetEmail("e1"), resetEmail("e2")}
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return(emails, nil)
		gomock.InOrder(
			d.mailer.EXPECT().Send(gomock.Any(), emails[0]).Return(nil),
			d.outbox.EXPECT().MarkSent(gomock.Any(), "e1").Return(nil),
			d.mailer.EXPECT().Send(gomock.Any(), emails[1]).Return(nil),
			d.outbox.EXPECT().MarkSent(gomock.Any(), "e2").Return(nil),
		)

		n, err := uc.SendDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 2, n)
	})

	t.Run("a failed send is rescheduled without payload values", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		e := resetEmail("e1")
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return([]entity.Email{e}, nil)
		// An error that echoes the message must not carry the token or the name into the database.
		d.mailer.EXPECT().Send(gomock.Any(), e).
			Return(errors.Join(errSMTP, errors.New("rejected body for Alice Liddell: token=s3cret-reset-token")))
		d.outbox.EXPECT().Reschedule(gomock.Any(), "e1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _, lastError string) error {
				assert.Contains(t, lastError, "451 try again later")
				assert.NotContains(t, lastError, e.Data.Token)
				assert.NotContains(t, lastError, e.Data.FullName)
				assert.Contains(t, lastError, "[redacted]")
				return nil
			})

		n, err := uc.SendDue(t.Context())
		require.NoError(t, err, "a failed send is recorded, not returned")
		assert.Equal(t, 1, n)
	})

	t.Run("a long error is truncated", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		e := resetEmail("e1")
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return([]entity.Email{e}, nil)
		d.mailer.EXPECT().Send(gomock.Any(), e).Return(errors.New(strings.Repeat("é", 5000)))
		d.outbox.EXPECT().Reschedule(gomock.Any(), "e1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _, lastError string) error {
				assert.LessOrEqual(t, utf8.RuneCountInString(lastError), 500)
				assert.True(t, utf8.ValidString(lastError))
				return nil
			})

		_, err := uc.SendDue(t.Context())
		require.NoError(t, err)
	})

	t.Run("shutdown finishes the email in flight and leaves the rest", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		ctx, cancel := context.WithCancel(t.Context())
		emails := []entity.Email{resetEmail("e1"), resetEmail("e2")}
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return(emails, nil)
		d.mailer.EXPECT().Send(gomock.Any(), emails[0]).DoAndReturn(func(ctx context.Context, _ entity.Email) error {
			cancel()
			assert.NoError(t, ctx.Err(), "the send in flight is not cancelled")
			return nil
		})
		d.outbox.EXPECT().MarkSent(gomock.Any(), "e1").DoAndReturn(func(ctx context.Context, _ string) error {
			assert.NoError(t, ctx.Err(), "the outcome is recorded after shutdown began")
			return nil
		})

		n, err := uc.SendDue(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
	})

	t.Run("claim failure", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return(nil, errDB)

		_, err := uc.SendDue(t.Context())
		require.ErrorIs(t, err, errDB)
	})

	t.Run("recording failure stops the batch", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		emails := []entity.Email{resetEmail("e1"), resetEmail("e2")}
		d.outbox.EXPECT().ClaimDue(gomock.Any()).Return(emails, nil)
		d.mailer.EXPECT().Send(gomock.Any(), emails[0]).Return(nil)
		d.outbox.EXPECT().MarkSent(gomock.Any(), "e1").Return(errDB)

		_, err := uc.SendDue(t.Context())
		require.ErrorIs(t, err, errDB)
	})
}

func TestMailMaintain(t *testing.T) {
	t.Parallel()

	near := func(want time.Time) gomock.Matcher {
		return gomock.Cond(func(got time.Time) bool { return got.Sub(want).Abs() < time.Minute })
	}

	t.Run("clears failed payloads after 1 hour and purges sent emails after 30 days", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		now := time.Now()
		d.outbox.EXPECT().ClearFailedPayloads(gomock.Any(), near(now.Add(-time.Hour))).Return(int64(2), nil)
		d.outbox.EXPECT().PurgeSent(gomock.Any(), near(now.Add(-30*24*time.Hour))).Return(int64(5), nil)

		require.NoError(t, uc.Maintain(t.Context()))
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		uc, d := newMail(t)
		d.outbox.EXPECT().ClearFailedPayloads(gomock.Any(), gomock.Any()).Return(int64(0), errDB)

		require.ErrorIs(t, uc.Maintain(t.Context()), errDB)
	})
}

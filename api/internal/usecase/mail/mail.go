// Package mail implements usecase.Mail: it sends the email outbox with at-least-once delivery (ADR 0010).
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

const (
	// _failedPayloadTTL is how long a failed email keeps its payload: a reset token expires after 1 hour.
	_failedPayloadTTL = time.Hour
	// _retention is how long a sent or failed email stays for audit: both keep the recipient.
	_retention = 30 * 24 * time.Hour
	// _maxErrorRunes bounds last_error (the column allows 1000 characters).
	_maxErrorRunes = 500
	_redacted      = "[redacted]"
	// _recordTimeout bounds each outcome write, which may run after shutdown began. With the mailer's
	// send timeout (at most 20 s, config) a batch of 10 stays well within its 5-minute lease.
	_recordTimeout = 5 * time.Second
)

// UseCase -.
type UseCase struct {
	tx     repo.Transactor
	outbox repo.OutboxRepo
	mailer repo.Mailer
	log    *slog.Logger
}

// New returns the Mail use case instrumented with OpenTelemetry tracing spans.
func New(tx repo.Transactor, outbox repo.OutboxRepo, mailer repo.Mailer, log *slog.Logger) usecase.Mail {
	return newTraced(&UseCase{tx: tx, outbox: outbox, mailer: mailer, log: log})
}

// SendDue -.
func (uc *UseCase) SendDue(ctx context.Context) (int, error) {
	// The claim commits on its own, so the lease holds while the emails are sent.
	var emails []entity.Email
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		var err error
		emails, err = uc.outbox.ClaimDue(ctx)
		return err
	}); err != nil {
		return 0, fmt.Errorf("MailUseCase - SendDue - claim: %w", err)
	}

	// Once ctx ends no further email starts and the rest are released for the next run. The email in
	// flight is finished even if ctx ends meanwhile; the mailer bounds the send.
	for i, e := range emails {
		if ctx.Err() != nil {
			return len(emails), uc.release(ctx, emails[i:])
		}
		if err := uc.send(context.WithoutCancel(ctx), e); err != nil {
			return len(emails), err
		}
	}
	return len(emails), nil
}

// release hands back emails claimed but not tried, so shutting down costs them no attempt.
func (uc *UseCase) release(ctx context.Context, emails []entity.Email) error {
	ids := make([]string, len(emails))
	for i, e := range emails {
		ids[i] = e.ID
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _recordTimeout)
	defer cancel()
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		return uc.outbox.Release(ctx, ids)
	}); err != nil {
		// Their lease expires instead.
		return fmt.Errorf("MailUseCase - SendDue - release: %w", err)
	}
	return nil
}

// send delivers one claimed email and records the outcome.
func (uc *UseCase) send(ctx context.Context, e entity.Email) error {
	sendErr := uc.mailer.Send(ctx, e)
	lastError := ""
	if sendErr != nil {
		lastError = sanitize(sendErr.Error(), e)
		uc.log.WarnContext(ctx, "email not sent, will retry",
			slog.String("email_id", e.ID), slog.String("kind", string(e.Kind)),
			slog.Int("attempt", e.Attempts), slog.String("error", lastError))
	}

	ctx, cancel := context.WithTimeout(ctx, _recordTimeout)
	defer cancel()
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		if sendErr == nil {
			return uc.outbox.MarkSent(ctx, e.ID)
		}
		return uc.outbox.Reschedule(ctx, e.ID, lastError)
	}); err != nil {
		// Sent but not marked: the lease expires and the email is sent again (at least once).
		return fmt.Errorf("MailUseCase - SendDue - record %s: %w", e.ID, err)
	}
	return nil
}

// Maintain -.
func (uc *UseCase) Maintain(ctx context.Context) error {
	now := time.Now()
	var cleared, sentPurged, failedPurged int64
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		var err error
		if cleared, err = uc.outbox.ClearFailedPayloads(ctx, now.Add(-_failedPayloadTTL)); err != nil {
			return err
		}
		if sentPurged, err = uc.outbox.PurgeSent(ctx, now.Add(-_retention)); err != nil {
			return err
		}
		failedPurged, err = uc.outbox.PurgeFailed(ctx, now.Add(-_retention))
		return err
	}); err != nil {
		return fmt.Errorf("MailUseCase - Maintain: %w", err)
	}
	uc.log.InfoContext(ctx, "email outbox maintained",
		slog.Int64("failed_payloads_cleared", cleared), slog.Int64("sent_purged", sentPurged),
		slog.Int64("failed_purged", failedPurged))
	return nil
}

// sanitize makes an error message safe to store and log: no payload value and no recipient (SMTP
// errors often echo it), valid UTF-8 without NUL (PostgreSQL text rejects it), bounded length.
func sanitize(msg string, e entity.Email) string {
	for _, v := range append(e.Data.Values(), e.To) {
		if v != "" {
			msg = strings.ReplaceAll(msg, v, _redacted)
		}
	}
	msg = strings.ReplaceAll(strings.ToValidUTF8(msg, "�"), "\x00", "")
	if r := []rune(msg); len(r) > _maxErrorRunes {
		msg = string(r[:_maxErrorRunes-1]) + "…"
	}
	return msg
}

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

const (
	_mailPollInterval    = 10 * time.Second
	_maintenanceInterval = 24 * time.Hour
)

// mailWorker sends the email outbox in the background (ADR 0010) and runs the daily maintenance.
type mailWorker struct {
	mail usecase.Mail
	log  *slog.Logger
	wake chan struct{}

	poll        time.Duration
	maintenance time.Duration
}

func newMailWorker(mail usecase.Mail, log *slog.Logger) *mailWorker {
	return &mailWorker{
		mail:        mail,
		log:         log,
		wake:        make(chan struct{}, 1),
		poll:        _mailPollInterval,
		maintenance: _maintenanceInterval,
	}
}

// Wake makes the worker look for due emails now instead of at its next poll. It never blocks; call
// it after the unit of work that enqueued an email has committed.
func (w *mailWorker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default: // a wake-up is already pending
	}
}

// Run sends due emails whenever it is woken and at every poll, and runs the maintenance at start and
// then daily, until ctx ends. It returns once the email in flight is finished.
func (w *mailWorker) Run(ctx context.Context) {
	poll := time.NewTicker(w.poll)
	defer poll.Stop()
	daily := time.NewTicker(w.maintenance)
	defer daily.Stop()

	w.maintain(ctx)
	for {
		w.sendDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-poll.C:
		case <-daily.C:
			w.maintain(ctx)
		}
	}
}

// sendDue claims and sends batches until none is due. A failure waits for the next wake-up or poll.
func (w *mailWorker) sendDue(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := w.mail.SendDue(ctx)
		if err != nil {
			if ctx.Err() == nil {
				w.log.ErrorContext(ctx, "sending emails failed", slog.Any("error", err))
			}
			return
		}
		if n == 0 {
			return
		}
	}
}

// maintain runs the daily clean-up steps. Each instance runs them; they are idempotent.
func (w *mailWorker) maintain(ctx context.Context) {
	if err := w.mail.Maintain(ctx); err != nil && ctx.Err() == nil {
		w.log.ErrorContext(ctx, "email outbox maintenance failed", slog.Any("error", err))
	}
}

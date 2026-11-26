package app

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _waitFor = 2 * time.Second

// fakeMail reports each SendDue call and returns the next batch size from batches (then 0).
type fakeMail struct {
	calls       chan struct{}
	batches     chan int
	release     chan struct{} // when set, SendDue waits for it, as a send in flight would
	maintenance atomic.Int32
}

func newFakeMail() *fakeMail {
	return &fakeMail{calls: make(chan struct{}, 100), batches: make(chan int, 100)}
}

func (f *fakeMail) SendDue(context.Context) (int, error) {
	f.calls <- struct{}{}
	if f.release != nil {
		<-f.release
	}
	select {
	case n := <-f.batches:
		return n, nil
	default:
		return 0, nil
	}
}

func (f *fakeMail) Maintain(context.Context) error {
	f.maintenance.Add(1)
	return nil
}

// newTestWorker never polls on its own, so every SendDue after the first comes from a wake-up.
func newTestWorker(mail *fakeMail, daily ...maintenanceStep) *worker {
	daily = append([]maintenanceStep{{name: "email outbox", run: mail.Maintain}}, daily...)
	w := newWorker(mail, slog.New(slog.DiscardHandler), daily...)
	w.poll, w.maintenance = time.Hour, time.Hour
	return w
}

func run(ctx context.Context, w *worker) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	return done
}

func waitCall(t *testing.T, mail *fakeMail) {
	t.Helper()
	select {
	case <-mail.calls:
	case <-time.After(_waitFor):
		require.FailNow(t, "SendDue was not called")
	}
}

func noCall(t *testing.T, mail *fakeMail) {
	t.Helper()
	select {
	case <-mail.calls:
		require.FailNow(t, "SendDue was called")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMailWorkerStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	mail := newFakeMail()
	ctx, cancel := context.WithCancel(t.Context())
	done := run(ctx, newTestWorker(mail))
	waitCall(t, mail)

	cancel()
	select {
	case <-done:
	case <-time.After(_waitFor):
		require.FailNow(t, "Run did not return after cancel")
	}
	assert.Equal(t, int32(1), mail.maintenance.Load(), "maintenance runs at start")
}

func TestMailWorkerFinishesTheEmailInFlight(t *testing.T) {
	t.Parallel()
	mail := newFakeMail()
	mail.release = make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	done := run(ctx, newTestWorker(mail))
	waitCall(t, mail)

	cancel()
	select {
	case <-done:
		require.FailNow(t, "Run returned while an email was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(mail.release)
	select {
	case <-done:
	case <-time.After(_waitFor):
		require.FailNow(t, "Run did not return after the email in flight")
	}
}

func TestMailWorkerWakesUpBeforeThePoll(t *testing.T) {
	t.Parallel()
	mail := newFakeMail()
	w := newTestWorker(mail)
	run(t.Context(), w)
	waitCall(t, mail) // the first round at start
	noCall(t, mail)

	w.Wake()
	waitCall(t, mail)

	// Wake-ups while one is pending collapse into one and never block.
	w.Wake()
	w.Wake()
	w.Wake()
	waitCall(t, mail)
}

func TestMailWorkerDrainsUntilNothingIsDue(t *testing.T) {
	t.Parallel()
	mail := newFakeMail()
	mail.batches <- 10
	mail.batches <- 3
	run(t.Context(), newTestWorker(mail))

	waitCall(t, mail) // 10 claimed
	waitCall(t, mail) // 3 claimed
	waitCall(t, mail) // none due
	noCall(t, mail)
}

func TestWorkerRunsEveryMaintenanceStepEvenAfterAFailure(t *testing.T) {
	t.Parallel()
	mail := newFakeMail()
	var purged atomic.Int32
	failing := maintenanceStep{name: "failing", run: func(context.Context) error { return errors.New("database unavailable") }}
	sessions := maintenanceStep{name: "sessions", run: func(context.Context) error { purged.Add(1); return nil }}
	ctx, cancel := context.WithCancel(t.Context())
	done := run(ctx, newTestWorker(mail, failing, sessions))
	waitCall(t, mail) // maintenance ran before the first send

	cancel()
	<-done
	assert.Equal(t, int32(1), mail.maintenance.Load())
	assert.Equal(t, int32(1), purged.Load(), "a failing step does not skip the next")
}

// Package outbox implements repo.OutboxRepo on PostgreSQL (ADR 0010).
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// payload is the stored form of entity.EmailData.
type payload struct {
	FullName string `json:"full_name,omitempty"`
	Token    string `json:"token,omitempty"`
}

// Repo -.
type Repo struct{}

// New returns an Outbox repository instrumented with OpenTelemetry tracing spans.
func New() repo.OutboxRepo {
	return newTraced(&Repo{})
}

// Enqueue -.
func (r *Repo) Enqueue(ctx context.Context, e entity.Email) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("OutboxRepo - Enqueue - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(e.ID)
	if err != nil {
		return fmt.Errorf("OutboxRepo - Enqueue - persistent.ParseID: %w", err)
	}
	data, err := json.Marshal(payload{FullName: e.Data.FullName, Token: e.Data.Token})
	if err != nil {
		return fmt.Errorf("OutboxRepo - Enqueue - json.Marshal: %w", err)
	}

	if err := q.EnqueueEmail(ctx, gen.EnqueueEmailParams{
		ID:      id,
		Kind:    string(e.Kind),
		ToEmail: e.To,
		Payload: data,
	}); err != nil {
		return fmt.Errorf("OutboxRepo - Enqueue - q.EnqueueEmail: %w", err)
	}
	return nil
}

// ClaimDue -.
func (r *Repo) ClaimDue(ctx context.Context) ([]entity.Email, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return nil, fmt.Errorf("OutboxRepo - ClaimDue - persistent.Queries: %w", err)
	}
	rows, err := q.ClaimDueEmails(ctx)
	if err != nil {
		return nil, fmt.Errorf("OutboxRepo - ClaimDue - q.ClaimDueEmails: %w", err)
	}

	emails := make([]entity.Email, 0, len(rows))
	for _, row := range rows {
		if row.Status != "pending" {
			continue // given up by the claim
		}
		var p payload
		if len(row.Payload) > 0 {
			if err := json.Unmarshal(row.Payload, &p); err != nil {
				return nil, fmt.Errorf("OutboxRepo - ClaimDue - json.Unmarshal %s: %w", row.ID, err)
			}
		}
		emails = append(emails, entity.Email{
			ID:       row.ID.String(),
			Kind:     entity.EmailKind(row.Kind),
			To:       row.ToEmail,
			Data:     entity.EmailData{FullName: p.FullName, Token: p.Token},
			Attempts: int(row.Attempts),
		})
	}
	return emails, nil
}

// Release -.
func (r *Repo) Release(ctx context.Context, ids []string) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("OutboxRepo - Release - persistent.Queries: %w", err)
	}
	uids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		if uids[i], err = persistent.ParseID(id); err != nil {
			return fmt.Errorf("OutboxRepo - Release - persistent.ParseID: %w", err)
		}
	}
	if err := q.ReleaseEmails(ctx, uids); err != nil {
		return fmt.Errorf("OutboxRepo - Release - q.ReleaseEmails: %w", err)
	}
	return nil
}

// MarkSent -.
func (r *Repo) MarkSent(ctx context.Context, id string) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("OutboxRepo - MarkSent - persistent.Queries: %w", err)
	}
	uid, err := persistent.ParseID(id)
	if err != nil {
		return fmt.Errorf("OutboxRepo - MarkSent - persistent.ParseID: %w", err)
	}
	if err := q.MarkEmailSent(ctx, uid); err != nil {
		return fmt.Errorf("OutboxRepo - MarkSent - q.MarkEmailSent: %w", err)
	}
	return nil
}

// Reschedule -.
func (r *Repo) Reschedule(ctx context.Context, id, lastError string) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("OutboxRepo - Reschedule - persistent.Queries: %w", err)
	}
	uid, err := persistent.ParseID(id)
	if err != nil {
		return fmt.Errorf("OutboxRepo - Reschedule - persistent.ParseID: %w", err)
	}
	if err := q.RescheduleEmail(ctx, gen.RescheduleEmailParams{ID: uid, LastError: lastError}); err != nil {
		return fmt.Errorf("OutboxRepo - Reschedule - q.RescheduleEmail: %w", err)
	}
	return nil
}

// ClearFailedPayloads -.
func (r *Repo) ClearFailedPayloads(ctx context.Context, createdBefore time.Time) (int64, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return 0, fmt.Errorf("OutboxRepo - ClearFailedPayloads - persistent.Queries: %w", err)
	}
	n, err := q.ClearFailedEmailPayloads(ctx, createdBefore)
	if err != nil {
		return 0, fmt.Errorf("OutboxRepo - ClearFailedPayloads - q.ClearFailedEmailPayloads: %w", err)
	}
	return n, nil
}

// PurgeSent -.
func (r *Repo) PurgeSent(ctx context.Context, sentBefore time.Time) (int64, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return 0, fmt.Errorf("OutboxRepo - PurgeSent - persistent.Queries: %w", err)
	}
	n, err := q.PurgeSentEmails(ctx, sentBefore)
	if err != nil {
		return 0, fmt.Errorf("OutboxRepo - PurgeSent - q.PurgeSentEmails: %w", err)
	}
	return n, nil
}

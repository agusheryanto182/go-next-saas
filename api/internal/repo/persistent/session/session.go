// Package session implements repo.SessionRepo on PostgreSQL (ADR 0006).
package session

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// ErrInvalidIP is returned for an IP address that does not parse.
var ErrInvalidIP = errors.New("invalid IP address")

// Repo -.
type Repo struct{}

// New returns a Session repository instrumented with OpenTelemetry tracing spans.
func New() repo.SessionRepo {
	return newTraced(&Repo{})
}

// Store -.
func (r *Repo) Store(ctx context.Context, s entity.Session, tokenHash []byte) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("SessionRepo - Store - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(s.ID)
	if err != nil {
		return fmt.Errorf("SessionRepo - Store - persistent.ParseID: %w", err)
	}
	userID, err := persistent.ParseID(s.UserID)
	if err != nil {
		return fmt.Errorf("SessionRepo - Store - persistent.ParseID user: %w", err)
	}
	ip, err := ipOrNull(s.IP)
	if err != nil {
		return fmt.Errorf("SessionRepo - Store: %w", err)
	}

	if err := q.CreateSession(ctx, gen.CreateSessionParams{
		ID:                id,
		UserID:            userID,
		TokenHash:         tokenHash,
		CreatedAt:         s.CreatedAt,
		IdleExpiresAt:     s.IdleExpiresAt,
		AbsoluteExpiresAt: s.AbsoluteExpiresAt,
		Ip:                ip,
		UserAgent:         persistent.TextOrNull(s.UserAgent),
	}); err != nil {
		return fmt.Errorf("SessionRepo - Store - q.CreateSession: %w", err)
	}
	return nil
}

// GetActiveByTokenHash -.
func (r *Repo) GetActiveByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (entity.Session, entity.User, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return entity.Session{}, entity.User{}, fmt.Errorf("SessionRepo - GetActiveByTokenHash - persistent.Queries: %w", err)
	}

	row, err := q.GetActiveSessionByTokenHash(ctx, gen.GetActiveSessionByTokenHashParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Session{}, entity.User{}, entity.ErrSessionNotFound
		}
		return entity.Session{}, entity.User{}, fmt.Errorf("SessionRepo - GetActiveByTokenHash - q.GetActiveSessionByTokenHash: %w", err)
	}

	s := entity.Session{
		ID:                row.ID.String(),
		UserID:            row.UserID.String(),
		CreatedAt:         row.CreatedAt,
		LastSeenAt:        row.LastSeenAt,
		IdleExpiresAt:     row.IdleExpiresAt,
		AbsoluteExpiresAt: row.AbsoluteExpiresAt,
		UserAgent:         persistent.TextOrEmpty(row.UserAgent),
	}
	if row.Ip != nil {
		s.IP = row.Ip.String()
	}
	u := entity.User{
		ID:        row.UserID.String(),
		Email:     row.Email,
		FullName:  row.FullName,
		Phone:     persistent.TextOrEmpty(row.Phone),
		CreatedAt: row.UserCreatedAt,
		UpdatedAt: row.UserUpdatedAt,
	}
	return s, u, nil
}

// Touch -.
func (r *Repo) Touch(ctx context.Context, id string, now time.Time) (bool, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return false, fmt.Errorf("SessionRepo - Touch - persistent.Queries: %w", err)
	}
	uid, err := persistent.ParseID(id)
	if err != nil {
		return false, fmt.Errorf("SessionRepo - Touch - persistent.ParseID: %w", err)
	}

	n, err := q.TouchSession(ctx, gen.TouchSessionParams{
		ID:            uid,
		Now:           now,
		IdleExpiresAt: now.Add(entity.SessionIdleTimeout),
		RefreshBefore: now.Add(-entity.SessionRefreshInterval),
	})
	if err != nil {
		return false, fmt.Errorf("SessionRepo - Touch - q.TouchSession: %w", err)
	}
	return n > 0, nil
}

// Revoke -.
func (r *Repo) Revoke(ctx context.Context, userID, id string, reason entity.RevokeReason, now time.Time) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("SessionRepo - Revoke - persistent.Queries: %w", err)
	}
	uid, err := persistent.ParseID(userID)
	if err != nil {
		return fmt.Errorf("SessionRepo - Revoke - persistent.ParseID user: %w", err)
	}
	sid, err := persistent.ParseID(id)
	if err != nil {
		return fmt.Errorf("SessionRepo - Revoke - persistent.ParseID: %w", err)
	}

	n, err := q.RevokeSession(ctx, gen.RevokeSessionParams{ID: sid, UserID: uid, Reason: string(reason), Now: now})
	if err != nil {
		return fmt.Errorf("SessionRepo - Revoke - q.RevokeSession: %w", err)
	}
	if n == 0 {
		return entity.ErrSessionNotFound
	}
	return nil
}

// RevokeOthers -.
func (r *Repo) RevokeOthers(ctx context.Context, userID, exceptID string, reason entity.RevokeReason, now time.Time) (int64, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - RevokeOthers - persistent.Queries: %w", err)
	}
	uid, err := persistent.ParseID(userID)
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - RevokeOthers - persistent.ParseID user: %w", err)
	}
	except, err := persistent.ParseID(exceptID)
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - RevokeOthers - persistent.ParseID: %w", err)
	}

	n, err := q.RevokeOtherSessions(ctx, gen.RevokeOtherSessionsParams{
		UserID: uid, ExceptID: except, Reason: string(reason), Now: now,
	})
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - RevokeOthers - q.RevokeOtherSessions: %w", err)
	}
	return n, nil
}

// PurgeEnded -.
func (r *Repo) PurgeEnded(ctx context.Context, endedBefore time.Time) (int64, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - PurgeEnded - persistent.Queries: %w", err)
	}
	n, err := q.PurgeEndedSessions(ctx, endedBefore)
	if err != nil {
		return 0, fmt.Errorf("SessionRepo - PurgeEnded - q.PurgeEndedSessions: %w", err)
	}
	return n, nil
}

// ipOrNull maps an empty IP to SQL NULL.
func ipOrNull(ip string) (*netip.Addr, error) {
	if ip == "" {
		return nil, nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("%w %q", ErrInvalidIP, ip)
	}
	return &addr, nil
}

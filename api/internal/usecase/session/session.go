// Package session implements usecase.Session: server-side sessions behind an opaque token (ADR 0006).
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

// _tokenBytes is the token's entropy: 256 bits, so a plain SHA-256 of it is safe to store.
const _tokenBytes = 32

// UseCase -.
type UseCase struct {
	tx       repo.Transactor
	sessions repo.SessionRepo
	now      func() time.Time
	log      *slog.Logger
}

// New returns the Session use case instrumented with OpenTelemetry tracing spans. now is the clock
// (time.Now in production).
func New(tx repo.Transactor, sessions repo.SessionRepo, now func() time.Time, log *slog.Logger) usecase.Session {
	return newTraced(&UseCase{tx: tx, sessions: sessions, now: now, log: log})
}

// Create -.
func (uc *UseCase) Create(ctx context.Context, userID, ip, userAgent string) (string, entity.Session, error) {
	token, hash := newToken()
	s := entity.NewSession(uuid.Must(uuid.NewV7()).String(), userID, uc.now(), ip, cleanUserAgent(userAgent))

	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		return uc.sessions.Store(ctx, s, hash)
	}); err != nil {
		return "", entity.Session{}, fmt.Errorf("SessionUseCase - Create: %w", err)
	}
	return token, s, nil
}

// Authenticate -.
func (uc *UseCase) Authenticate(ctx context.Context, token string) (entity.Session, entity.User, error) {
	hash, ok := hashToken(token)
	if !ok {
		return entity.Session{}, entity.User{}, entity.ErrSessionNotFound
	}
	now := uc.now()

	var (
		s entity.Session
		u entity.User
	)
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		var err error
		s, u, err = uc.sessions.GetActiveByTokenHash(ctx, hash, now)
		return err
	}); err != nil {
		if errors.Is(err, entity.ErrSessionNotFound) {
			return entity.Session{}, entity.User{}, entity.ErrSessionNotFound
		}
		return entity.Session{}, entity.User{}, fmt.Errorf("SessionUseCase - Authenticate: %w", err)
	}
	if s.NeedsRefresh(now) {
		uc.refresh(ctx, s.ID, now)
	}
	return s, u, nil
}

// refresh extends the idle expiry in its own unit of work. It is best effort: the session is valid
// either way, and a failed write (lock or statement timeout) must not fail the request; the next
// request tries again.
func (uc *UseCase) refresh(ctx context.Context, sessionID string, now time.Time) {
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		// A concurrent request may have refreshed it already; then this writes nothing.
		_, err := uc.sessions.Touch(ctx, sessionID, now)
		return err
	}); err != nil {
		uc.log.WarnContext(ctx, "session refresh failed", slog.String("session_id", sessionID), slog.Any("error", err))
	}
}

// Revoke -.
func (uc *UseCase) Revoke(ctx context.Context, userID, sessionID string, reason entity.RevokeReason) error {
	now := uc.now()
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		return uc.sessions.Revoke(ctx, userID, sessionID, reason, now)
	}); err != nil {
		if errors.Is(err, entity.ErrSessionNotFound) {
			return entity.ErrSessionNotFound
		}
		return fmt.Errorf("SessionUseCase - Revoke: %w", err)
	}
	return nil
}

// RevokeOthers -.
func (uc *UseCase) RevokeOthers(ctx context.Context, userID, exceptSessionID string, reason entity.RevokeReason) error {
	now := uc.now()
	var n int64
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		var err error
		n, err = uc.sessions.RevokeOthers(ctx, userID, exceptSessionID, reason, now)
		return err
	}); err != nil {
		return fmt.Errorf("SessionUseCase - RevokeOthers: %w", err)
	}
	uc.log.InfoContext(ctx, "sessions revoked",
		slog.String("user_id", userID), slog.String("reason", string(reason)), slog.Int64("count", n))
	return nil
}

// Purge -.
func (uc *UseCase) Purge(ctx context.Context) error {
	endedBefore := uc.now().Add(-entity.SessionRetention)
	var n int64
	if err := uc.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		var err error
		n, err = uc.sessions.PurgeEnded(ctx, endedBefore)
		return err
	}); err != nil {
		return fmt.Errorf("SessionUseCase - Purge: %w", err)
	}
	uc.log.InfoContext(ctx, "ended sessions purged", slog.Int64("count", n))
	return nil
}

// tokenEncoding is base64url without padding, strict so that each token has exactly one spelling.
func tokenEncoding() *base64.Encoding { return base64.RawURLEncoding.Strict() }

// newToken returns a fresh token and the hash to store. crypto/rand.Read never fails: on a broken
// random source the program crashes rather than issue a guessable token.
func newToken() (token string, hash []byte) {
	b := make([]byte, _tokenBytes)
	_, _ = rand.Read(b)
	sum := sha256.Sum256(b)
	return tokenEncoding().EncodeToString(b), sum[:]
}

// hashToken returns the stored form of token, or false if token cannot be one this package issued.
func hashToken(token string) ([]byte, bool) {
	enc := tokenEncoding()
	if enc.EncodedLen(_tokenBytes) != len(token) {
		return nil, false
	}
	b, err := enc.DecodeString(token)
	if err != nil || len(b) != _tokenBytes {
		return nil, false
	}
	sum := sha256.Sum256(b)
	return sum[:], true
}

// cleanUserAgent makes the header safe to store: valid UTF-8 without NUL (PostgreSQL text rejects
// it), at most entity.SessionUserAgentMaxRunes.
func cleanUserAgent(ua string) string {
	ua = strings.ReplaceAll(strings.ToValidUTF8(ua, "�"), "\x00", "")
	if r := []rune(ua); len(r) > entity.SessionUserAgentMaxRunes {
		ua = string(r[:entity.SessionUserAgentMaxRunes])
	}
	return ua
}

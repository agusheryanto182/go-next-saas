package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

// signIn is a fixed test clock.
func signIn() time.Time { return time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC) }

func TestNewSessionCountsBothExpiriesFromSignIn(t *testing.T) {
	t.Parallel()
	s := entity.NewSession("s1", "u1", signIn(), "203.0.113.7", "Firefox")

	assert.Equal(t, signIn(), s.CreatedAt)
	assert.Equal(t, signIn(), s.LastSeenAt)
	assert.Equal(t, signIn().Add(7*24*time.Hour), s.IdleExpiresAt)
	assert.Equal(t, signIn().Add(30*24*time.Hour), s.AbsoluteExpiresAt)
	assert.True(t, s.RevokedAt.IsZero())
}

func TestSessionNeedsRefresh(t *testing.T) {
	t.Parallel()
	s := entity.NewSession("s1", "u1", signIn(), "", "")

	tests := []struct {
		name  string
		after time.Duration
		want  bool
	}{
		{"just signed in", 0, false},
		{"59 minutes later", 59 * time.Minute, false},
		{"exactly an hour later", time.Hour, false},
		{"just over an hour later", time.Hour + time.Second, true},
		{"a day later", 24 * time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, s.NeedsRefresh(signIn().Add(tt.after)))
		})
	}
}

func TestSessionActive(t *testing.T) {
	t.Parallel()
	fresh := entity.NewSession("s1", "u1", signIn(), "", "")
	revoked := fresh
	revoked.RevokedAt, revoked.RevokedReason = signIn().Add(time.Minute), entity.RevokeLogout
	// Used every day: the idle expiry keeps moving, the absolute one does not.
	busy := fresh
	busy.IdleExpiresAt = fresh.AbsoluteExpiresAt

	tests := []struct {
		name string
		s    entity.Session
		at   time.Time
		want bool
	}{
		{"fresh", fresh, signIn().Add(time.Hour), true},
		{"idle for 7 days", fresh, fresh.IdleExpiresAt, false},
		{"revoked", revoked, signIn().Add(time.Hour), false},
		{"busy, before the absolute expiry", busy, fresh.AbsoluteExpiresAt.Add(-time.Second), true},
		{"busy, at the absolute expiry", busy, fresh.AbsoluteExpiresAt, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.s.Active(tt.at))
		})
	}
}

func TestSessionMaxAgeIsTheRemainingAbsoluteLifetime(t *testing.T) {
	t.Parallel()
	s := entity.NewSession("s1", "u1", signIn(), "", "")

	assert.Equal(t, 30*24*time.Hour, s.MaxAge(signIn()))
	assert.Equal(t, 20*24*time.Hour, s.MaxAge(signIn().Add(10*24*time.Hour)))
	assert.Zero(t, s.MaxAge(signIn().Add(31*24*time.Hour)), "never negative")
}

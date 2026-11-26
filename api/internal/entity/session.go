package entity

import "time"

// Session lifetimes (ADR 0006).
const (
	// SessionIdleTimeout ends a session that has not been used for this long.
	SessionIdleTimeout = 7 * 24 * time.Hour
	// SessionAbsoluteLifetime ends every session this long after sign-in, however active.
	SessionAbsoluteLifetime = 30 * 24 * time.Hour
	// SessionRefreshInterval is how often use extends the idle expiry: at most one write an hour.
	SessionRefreshInterval = time.Hour
	// SessionRetention is how long an ended session stays listed before it is purged.
	SessionRetention = 90 * 24 * time.Hour
	// SessionUserAgentMaxRunes bounds the stored user agent (the sessions_user_agent_length check).
	SessionUserAgentMaxRunes = 512
)

// RevokeReason says why a session was ended before it expired (ADR 0006).
type RevokeReason string

// Revocation reasons; the sessions_revoked_reason_valid check lists the same values.
const (
	RevokeLogout             RevokeReason = "logout"
	RevokePasswordChanged    RevokeReason = "password_changed"
	RevokePasswordReset      RevokeReason = "password_reset"
	RevokeSignedOutElsewhere RevokeReason = "signed_out_elsewhere"
)

// Session is a signed-in device (ADR 0006). Its ID is the public handle for listing and revoking;
// the token that authenticates it is never stored, only its SHA-256.
type Session struct {
	ID                string
	UserID            string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	// RevokedAt is zero while the session has not been revoked.
	RevokedAt     time.Time
	RevokedReason RevokeReason
	IP            string // empty when unknown
	UserAgent     string // empty when not sent
}

// NewSession starts a session for userID at now, with both expiries counted from now.
func NewSession(id, userID string, now time.Time, ip, userAgent string) Session {
	return Session{
		ID:                id,
		UserID:            userID,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     now.Add(SessionIdleTimeout),
		AbsoluteExpiresAt: now.Add(SessionAbsoluteLifetime),
		IP:                ip,
		UserAgent:         userAgent,
	}
}

// Active reports whether the session authenticates requests at now: not revoked and neither expiry
// reached.
func (s Session) Active(now time.Time) bool {
	return s.RevokedAt.IsZero() && now.Before(s.IdleExpiresAt) && now.Before(s.AbsoluteExpiresAt)
}

// NeedsRefresh reports whether a request at now should extend the idle expiry: the last extension
// is more than SessionRefreshInterval ago. The repository applies the same condition in its UPDATE,
// so concurrent requests write at most once.
func (s Session) NeedsRefresh(now time.Time) bool {
	return now.Sub(s.LastSeenAt) > SessionRefreshInterval
}

// MaxAge is how long the session cookie should live from now: the remaining absolute lifetime. The
// idle limit is enforced on the server (ADR 0006).
func (s Session) MaxAge(now time.Time) time.Duration {
	return max(s.AbsoluteExpiresAt.Sub(now), 0)
}

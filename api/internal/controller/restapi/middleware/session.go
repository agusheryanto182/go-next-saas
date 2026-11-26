package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

// SessionCookieName is the session cookie (ADR 0006). The __Host- prefix makes browsers insist on
// Secure, Path=/ and no Domain, so no subdomain can set or read it.
const SessionCookieName = "__Host-session"

type sessionKey struct{}

type signedIn struct {
	session entity.Session
	user    entity.User
}

// Session resolves the session cookie, if any, and puts the session and its user in the request
// context (read them with SessionFrom). A missing or invalid cookie leaves the request anonymous:
// which operations need a session is the contract's decision, enforced by the request validator.
func Session(sessions usecase.Session, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			s, u, err := sessions.Authenticate(r.Context(), cookie.Value)
			switch {
			case err == nil:
				r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, signedIn{session: s, user: u}))
			case errors.Is(err, entity.ErrSessionNotFound):
			default:
				// The error never holds the token: the use case passes only its hash on.
				log.ErrorContext(r.Context(), "session lookup failed", slog.Any("error", err))
				problem.Write(w, problem.New(http.StatusInternalServerError, "Something went wrong. Please try again."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SessionFrom returns the caller's session and user, or false when the request is anonymous.
func SessionFrom(ctx context.Context) (entity.Session, entity.User, bool) {
	in, ok := ctx.Value(sessionKey{}).(signedIn)
	return in.session, in.user, ok
}

// NewSessionCookie is the cookie that carries token for maxAge: the session's remaining absolute
// lifetime (entity.Session.MaxAge). A lifetime under a second yields an expired cookie.
func NewSessionCookie(token string, maxAge time.Duration) *http.Cookie {
	seconds := int(maxAge / time.Second)
	if seconds <= 0 {
		return ClearedSessionCookie()
	}
	return sessionCookie(token, seconds)
}

// ClearedSessionCookie makes the browser delete the session cookie.
func ClearedSessionCookie() *http.Cookie {
	return sessionCookie("", -1) // sent as Max-Age=0
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

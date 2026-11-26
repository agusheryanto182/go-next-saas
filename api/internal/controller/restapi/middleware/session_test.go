package middleware_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
)

// The headers are the contract's SessionCookie and ClearedSessionCookie (openapi.yaml).
func TestSessionCookies(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "__Host-session=tok; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax",
		middleware.NewSessionCookie("tok", 30*24*time.Hour).String())
	assert.Equal(t, "__Host-session=tok; Path=/; Max-Age=59; HttpOnly; Secure; SameSite=Lax",
		middleware.NewSessionCookie("tok", 59*time.Second+900*time.Millisecond).String(), "rounded down, never outliving the session")

	cleared := "__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	assert.Equal(t, cleared, middleware.ClearedSessionCookie().String())
	assert.Equal(t, cleared, middleware.NewSessionCookie("tok", 500*time.Millisecond).String(), "an ended session clears the cookie")
}

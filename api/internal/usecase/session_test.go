package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase/session"
)

type sessionDeps struct {
	tx       *MockTransactor
	sessions *MockSessionRepo
}

// sessionNow is the fixed clock of the session use case under test.
func sessionNow() time.Time { return time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC) }

func newSession(t *testing.T) (usecase.Session, sessionDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := sessionDeps{tx: NewMockTransactor(ctrl), sessions: NewMockSessionRepo(ctrl)}
	// The sessions table is global identity data: every unit of work has an empty scope.
	d.tx.EXPECT().InScope(gomock.Any(), repo.Scope{}, gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, _ repo.Scope, fn func(context.Context) error) error { return fn(ctx) })
	return session.New(d.tx, d.sessions, sessionNow, slog.New(slog.DiscardHandler)), d
}

func sha(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// create runs Create with a repository that accepts the session, and returns what was stored.
func create(t *testing.T, uc usecase.Session, d sessionDeps, ip, userAgent string) (string, entity.Session, []byte) {
	t.Helper()
	var stored entity.Session
	var hash []byte
	d.sessions.EXPECT().Store(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, s entity.Session, h []byte) error {
			stored, hash = s, h
			return nil
		})
	token, s, err := uc.Create(t.Context(), "u1", ip, userAgent)
	require.NoError(t, err)
	assert.Equal(t, stored, s, "the returned session is the stored one")
	return token, s, hash
}

func TestSessionCreate(t *testing.T) {
	t.Parallel()

	t.Run("token is 32 random bytes in base64url, and only its SHA-256 is stored", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		token, s, hash := create(t, uc, d, "203.0.113.7", "Firefox")

		assert.Len(t, token, 43, "32 bytes, base64url without padding")
		raw, err := base64.RawURLEncoding.DecodeString(token)
		require.NoError(t, err)
		assert.Len(t, raw, 32)
		assert.Equal(t, sha(raw), hash)
		assert.NotContains(t, string(hash), token)

		assert.Equal(t, entity.NewSession(s.ID, "u1", sessionNow(), "203.0.113.7", "Firefox"), s)
		assert.Len(t, s.ID, 36)
	})

	t.Run("every session gets a new token", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		first, _, _ := create(t, uc, d, "", "")
		second, _, _ := create(t, uc, d, "", "")
		assert.NotEqual(t, first, second)
	})

	t.Run("the user agent is cut to 512 characters and made storable", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		_, long, _ := create(t, uc, d, "", strings.Repeat("é", 600))
		assert.Equal(t, 512, utf8.RuneCountInString(long.UserAgent))

		_, odd, _ := create(t, uc, d, "", "Mozilla\x00/5.0 \xff")
		assert.True(t, utf8.ValidString(odd.UserAgent))
		assert.NotContains(t, odd.UserAgent, "\x00")
	})

	t.Run("a storage failure returns no token", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().Store(gomock.Any(), gomock.Any(), gomock.Any()).Return(errDB)

		token, _, err := uc.Create(t.Context(), "u1", "", "")
		require.ErrorIs(t, err, errDB)
		assert.Empty(t, token)
	})
}

func TestSessionAuthenticate(t *testing.T) {
	t.Parallel()
	zeros := make([]byte, 32)
	token := base64.RawURLEncoding.EncodeToString(zeros)
	user := entity.User{ID: "u1", Email: "alice@acme.test", FullName: "Alice"}
	seenAgo := func(d time.Duration) entity.Session {
		s := entity.NewSession("s1", "u1", sessionNow().Add(-d), "", "")
		return s
	}

	t.Run("an active session seen within the hour is not written", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		want := seenAgo(10 * time.Minute)
		d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), sha(zeros), sessionNow()).Return(want, user, nil)

		s, u, err := uc.Authenticate(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t, want, s)
		assert.Equal(t, user, u)
	})

	t.Run("an active session last seen over an hour ago is refreshed", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), sha(zeros), sessionNow()).Return(seenAgo(2*time.Hour), user, nil)
		d.sessions.EXPECT().Touch(gomock.Any(), "s1", sessionNow()).Return(true, nil)

		_, _, err := uc.Authenticate(t.Context(), token)
		require.NoError(t, err)
	})

	t.Run("a refresh another request already made is fine", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), gomock.Any(), gomock.Any()).Return(seenAgo(2*time.Hour), user, nil)
		d.sessions.EXPECT().Touch(gomock.Any(), "s1", sessionNow()).Return(false, nil)

		_, _, err := uc.Authenticate(t.Context(), token)
		require.NoError(t, err)
	})

	t.Run("an unknown, revoked or expired session is not found", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.Session{}, entity.User{}, entity.ErrSessionNotFound)

		_, _, err := uc.Authenticate(t.Context(), token)
		assert.Equal(t, entity.ErrSessionNotFound, err, "the sentinel itself, unwrapped")
	})

	t.Run("a database failure is not reported as a missing session", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.Session{}, entity.User{}, errDB)

		_, _, err := uc.Authenticate(t.Context(), token)
		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, entity.ErrSessionNotFound)
	})

	// None of these can be a token Create issued, so they never reach the database (the mock
	// fails the test on any call).
	malformed := map[string]string{
		"empty":                     "",
		"too short":                 "abc",
		"one byte short":            base64.RawURLEncoding.EncodeToString(zeros[:31]),
		"with padding":              base64.URLEncoding.EncodeToString(zeros),
		"standard base64 alphabet":  strings.Repeat("+", 43),
		"non-canonical final digit": strings.Repeat("A", 42) + "B",
		"too long":                  token + "A",
	}
	for name, bad := range malformed {
		t.Run("malformed token: "+name, func(t *testing.T) {
			t.Parallel()
			uc, _ := newSession(t)
			_, _, err := uc.Authenticate(t.Context(), bad)
			assert.Equal(t, entity.ErrSessionNotFound, err)
		})
	}
}

func TestSessionRevoke(t *testing.T) {
	t.Parallel()

	t.Run("one session of the user, at the clock's time", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().Revoke(gomock.Any(), "u1", "s1", entity.RevokeLogout, sessionNow()).Return(nil)
		require.NoError(t, uc.Revoke(t.Context(), "u1", "s1", entity.RevokeLogout))
	})

	t.Run("no such active session", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().Revoke(gomock.Any(), "u1", "s1", entity.RevokeSignedOutElsewhere, sessionNow()).
			Return(entity.ErrSessionNotFound)
		assert.Equal(t, entity.ErrSessionNotFound, uc.Revoke(t.Context(), "u1", "s1", entity.RevokeSignedOutElsewhere))
	})

	t.Run("all others of the user", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().RevokeOthers(gomock.Any(), "u1", "s1", entity.RevokePasswordChanged, sessionNow()).Return(int64(3), nil)
		require.NoError(t, uc.RevokeOthers(t.Context(), "u1", "s1", entity.RevokePasswordChanged))
	})

	t.Run("a failure to revoke others is reported", func(t *testing.T) {
		t.Parallel()
		uc, d := newSession(t)
		d.sessions.EXPECT().RevokeOthers(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errDB)
		require.ErrorIs(t, uc.RevokeOthers(t.Context(), "u1", "s1", entity.RevokePasswordChanged), errDB)
	})
}

func TestSessionPurgeKeepsEndedSessionsFor90Days(t *testing.T) {
	t.Parallel()
	uc, d := newSession(t)
	d.sessions.EXPECT().PurgeEnded(gomock.Any(), sessionNow().Add(-90*24*time.Hour)).Return(int64(2), nil)
	require.NoError(t, uc.Purge(t.Context()))
}

// Spans leave the system; the token must not be in any of them (ADR 0006, api/AGENTS.md).
//
//nolint:paralleltest // replaces the global tracer provider for the duration of the test
func TestSessionTokenNeverInSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	uc, d := newSession(t)
	token, s, _ := create(t, uc, d, "203.0.113.7", "Firefox")
	d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), gomock.Any(), gomock.Any()).Return(s, entity.User{ID: "u1"}, nil)
	_, _, err := uc.Authenticate(t.Context(), token)
	require.NoError(t, err)
	d.sessions.EXPECT().GetActiveByTokenHash(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.Session{}, entity.User{}, errDB)
	_, _, err = uc.Authenticate(t.Context(), token)
	require.Error(t, err)

	spans := recorder.Ended()
	require.Len(t, spans, 3)
	for _, span := range spans {
		for _, attr := range span.Attributes() {
			assert.NotContains(t, attr.Value.String(), token, "span %s attribute %s", span.Name(), attr.Key)
		}
		for _, event := range span.Events() {
			for _, attr := range event.Attributes {
				assert.NotContains(t, attr.Value.String(), token, "span %s event %s", span.Name(), event.Name)
			}
		}
		assert.NotContains(t, span.Status().Description, token)
	}
}

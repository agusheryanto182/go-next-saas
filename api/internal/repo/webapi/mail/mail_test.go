package mail_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/webapi/mail"
)

func validConfig() mail.Config {
	return mail.Config{
		Host: "localhost", Port: 1025, TLS: mail.TLSStartTLS,
		ConnectTimeout: time.Second, SendTimeout: time.Second,
		From: "go-next-saas <no-reply@go-next-saas.test>", AppBaseURL: "http://localhost:3000",
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	for _, mode := range []mail.TLSMode{mail.TLSNone, mail.TLSStartTLS, mail.TLSImplicit} {
		cfg := validConfig()
		cfg.TLS = mode
		_, err := mail.New(cfg)
		require.NoError(t, err, mode)
	}

	cfg := validConfig()
	cfg.TLS = "false"
	_, err := mail.New(cfg)
	require.ErrorIs(t, err, mail.ErrInvalidTLSMode)

	cfg = validConfig()
	cfg.From = "go-next-saas"
	_, err = mail.New(cfg)
	require.ErrorIs(t, err, mail.ErrInvalidFrom)
}

func TestSendUnknownKind(t *testing.T) {
	t.Parallel()
	m, err := mail.New(validConfig())
	require.NoError(t, err)

	err = m.Send(t.Context(), entity.Email{ID: "e1", Kind: "invitation", To: "a@acme.test"})
	require.ErrorIs(t, err, mail.ErrUnknownKind)
}

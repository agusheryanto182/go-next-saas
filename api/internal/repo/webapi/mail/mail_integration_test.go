//go:build integration

package mail_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/webapi/mail"
)

// The same image as compose.yml.
const _mailpitImage = "axllent/mailpit:v1.33.1"

type mailpitMessage struct {
	MessageID string
	Subject   string
	From      struct{ Name, Address string }
	To        []struct{ Address string }
	Text      string
	HTML      string
}

// startMailpit returns the SMTP port and the base URL of the Mailpit API.
func startMailpit(t *testing.T) (int, string) {
	t.Helper()
	ctx := t.Context()
	c, err := testcontainers.Run(ctx, _mailpitImage,
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/readyz").WithPort("8025/tcp").WithStartupTimeout(60*time.Second)),
	)
	testcontainers.CleanupContainer(t, c)
	require.NoError(t, err, "start mailpit")

	host, err := c.Host(ctx)
	require.NoError(t, err)
	smtpPort, err := c.MappedPort(ctx, "1025/tcp")
	require.NoError(t, err)
	apiPort, err := c.MappedPort(ctx, "8025/tcp")
	require.NoError(t, err)
	return int(smtpPort.Num()), "http://" + host + ":" + apiPort.Port()
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode, url)
	require.NoError(t, json.NewDecoder(res.Body).Decode(v))
}

// received finds the message with the given Message-ID (without angle brackets).
func received(t *testing.T, api, messageID string) mailpitMessage {
	t.Helper()
	var list struct {
		Messages []struct{ ID, MessageID string }
	}
	getJSON(t, api+"/api/v1/messages", &list)
	for _, m := range list.Messages {
		if m.MessageID == messageID {
			var msg mailpitMessage
			getJSON(t, api+"/api/v1/message/"+m.ID, &msg)
			return msg
		}
	}
	require.FailNow(t, "message not received", messageID)
	return mailpitMessage{}
}

func TestMailerAgainstMailpit(t *testing.T) {
	t.Parallel()
	smtpPort, api := startMailpit(t)

	mailer, err := mail.New(mail.Config{
		Host: "localhost", Port: smtpPort, TLS: mail.TLSNone,
		ConnectTimeout: 5 * time.Second, SendTimeout: 10 * time.Second,
		From:       "go-next-saas <no-reply@go-next-saas.test>",
		AppBaseURL: "https://app.example.test/",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		email       entity.Email
		subject     string
		wantText    []string
		wantHTML    []string
		notInBodies []string
	}{
		{
			name: "password reset",
			email: entity.Email{
				Kind: entity.EmailPasswordReset, To: "alice@acme.test",
				Data: entity.EmailData{FullName: "Alice", Token: "tok_Ab-9"},
			},
			subject:  "Reset your password",
			wantText: []string{"Hi Alice,", "https://app.example.test/reset-password#token=tok_Ab-9", "valid for 1 hour"},
			wantHTML: []string{`<a href="https://app.example.test/reset-password#token=tok_Ab-9">`, "valid for 1 hour"},
		},
		{
			name: "password changed",
			email: entity.Email{
				Kind: entity.EmailPasswordChanged, To: "bob@globex.test",
				Data: entity.EmailData{FullName: "Bob <script>"},
			},
			subject:     "Your password was changed",
			wantText:    []string{"Hi Bob <script>,", "https://app.example.test"},
			wantHTML:    []string{"Hi Bob &lt;script&gt;,", `<a href="https://app.example.test">`},
			notInBodies: []string{"reset-password#token"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.email.ID = uuid.Must(uuid.NewV7()).String()
			require.NoError(t, mailer.Send(t.Context(), tt.email))

			msg := received(t, api, tt.email.ID+"@go-next-saas.test")
			assert.Equal(t, tt.subject, msg.Subject)
			assert.Equal(t, "no-reply@go-next-saas.test", msg.From.Address)
			require.Len(t, msg.To, 1)
			assert.Equal(t, tt.email.To, msg.To[0].Address)
			require.NotEmpty(t, msg.Text, "text part")
			require.NotEmpty(t, msg.HTML, "HTML part")
			for _, s := range tt.wantText {
				assert.Contains(t, msg.Text, s, "text part")
			}
			for _, s := range tt.wantHTML {
				assert.Contains(t, msg.HTML, s, "HTML part")
			}
			for _, s := range tt.notInBodies {
				assert.NotContains(t, msg.Text+msg.HTML, s)
			}
		})
	}

	t.Run("unreachable server fails within the connect timeout", func(t *testing.T) {
		t.Parallel()
		down, err := mail.New(mail.Config{
			Host: "localhost", Port: 1, TLS: mail.TLSNone,
			ConnectTimeout: time.Second, SendTimeout: 2 * time.Second,
			From: "no-reply@go-next-saas.test", AppBaseURL: "https://app.example.test",
		})
		require.NoError(t, err)
		start := time.Now()
		err = down.Send(t.Context(), entity.Email{ID: uuid.NewString(), Kind: entity.EmailPasswordChanged, To: "a@acme.test"})
		require.Error(t, err)
		assert.Less(t, time.Since(start), 3*time.Second, fmt.Sprint(err))
	})
}

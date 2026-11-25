// Package mail implements repo.Mailer over SMTP with go-mail (ADR 0010). Each kind of email has an
// embedded text template (templates/<kind>.txt, which also defines "subject") and an HTML template
// (templates/<kind>.html); every email is sent with both parts.
package mail

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	netmail "net/mail"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"

	gomail "github.com/wneessen/go-mail"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
)

//go:embed templates
var _templates embed.FS

// TLSMode is how the SMTP connection is encrypted.
type TLSMode string

// TLS modes. Production providers use STARTTLS on port 587; TLS is implicit TLS, usually on 465.
const (
	TLSNone     TLSMode = "none"
	TLSStartTLS TLSMode = "starttls"
	TLSImplicit TLSMode = "tls"
)

var (
	// ErrInvalidTLSMode is returned by New for an unknown TLSMode.
	ErrInvalidTLSMode = errors.New("invalid SMTP TLS mode")
	// ErrInvalidFrom is returned by New for a From that is not a single email address.
	ErrInvalidFrom = errors.New("invalid sender address")
	// ErrUnknownKind is returned by Send for an email kind without templates.
	ErrUnknownKind = errors.New("unknown email kind")
)

// Config is how to reach the SMTP server and what to put in emails.
type Config struct {
	Host     string
	Port     int
	Username string // empty: no authentication
	Password string
	TLS      TLSMode
	// ConnectTimeout bounds connecting to the server, SendTimeout one whole delivery.
	ConnectTimeout time.Duration
	SendTimeout    time.Duration
	// From is the sender, e.g. `go-next-saas <no-reply@example.com>`.
	From string
	// AppBaseURL is the web app's public URL; links in emails point there.
	AppBaseURL string
}

type templates struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

// templateData is what every template receives.
type templateData struct {
	FullName string
	AppURL   string
	// Link is the action link (password_reset).
	Link string
}

// Mailer -.
type Mailer struct {
	client      *gomail.Client
	from        string
	domain      string // of From, for Message-IDs
	appBaseURL  string
	sendTimeout time.Duration
	templates   map[entity.EmailKind]templates
}

// New parses the templates and configures the SMTP client. It does not connect: each Send dials.
func New(cfg Config) (repo.Mailer, error) {
	from, err := netmail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("Mailer - New - from: %w: %w", ErrInvalidFrom, err)
	}
	_, domain, _ := strings.Cut(from.Address, "@")

	opts := []gomail.Option{gomail.WithPort(cfg.Port), gomail.WithTimeout(cfg.ConnectTimeout)}
	switch cfg.TLS {
	case TLSNone:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case TLSStartTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	case TLSImplicit:
		opts = append(opts, gomail.WithSSL())
	default:
		return nil, fmt.Errorf("Mailer - New - %w %q", ErrInvalidTLSMode, cfg.TLS)
	}
	if cfg.Username != "" {
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(cfg.Username), gomail.WithPassword(cfg.Password))
	}
	client, err := gomail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("Mailer - New - gomail.NewClient: %w", err)
	}

	m := &Mailer{
		client:      client,
		from:        from.String(),
		domain:      domain,
		appBaseURL:  strings.TrimRight(cfg.AppBaseURL, "/"),
		sendTimeout: cfg.SendTimeout,
		templates:   make(map[entity.EmailKind]templates),
	}
	for _, kind := range []entity.EmailKind{entity.EmailPasswordReset, entity.EmailPasswordChanged} {
		text, err := texttemplate.ParseFS(_templates, "templates/"+string(kind)+".txt")
		if err != nil {
			return nil, fmt.Errorf("Mailer - New - text template %s: %w", kind, err)
		}
		html, err := htmltemplate.ParseFS(_templates, "templates/"+string(kind)+".html")
		if err != nil {
			return nil, fmt.Errorf("Mailer - New - HTML template %s: %w", kind, err)
		}
		m.templates[kind] = templates{text: text, html: html}
	}
	return m, nil
}

// Send renders e and delivers it within the send timeout.
// There is no tracing span here: SMTP errors can name the recipient, and spans never carry emails.
func (m *Mailer) Send(ctx context.Context, e entity.Email) error {
	msg, err := m.message(e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, m.sendTimeout)
	defer cancel()
	if err := m.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("Mailer - Send - client.DialAndSendWithContext: %w", err)
	}
	return nil
}

// message renders e into a multipart message with a text and an HTML part.
func (m *Mailer) message(e entity.Email) (*gomail.Msg, error) {
	t, ok := m.templates[e.Kind]
	if !ok {
		return nil, fmt.Errorf("Mailer - Send - %w %q", ErrUnknownKind, e.Kind)
	}
	data := templateData{FullName: e.Data.FullName, AppURL: m.appBaseURL}
	if e.Kind == entity.EmailPasswordReset {
		data.Link = resetLink(m.appBaseURL, e.Data.Token)
	}

	var subject, text, html bytes.Buffer
	if err := t.text.ExecuteTemplate(&subject, "subject", data); err != nil {
		return nil, fmt.Errorf("Mailer - Send - subject: %w", err)
	}
	if err := t.text.Execute(&text, data); err != nil {
		return nil, fmt.Errorf("Mailer - Send - text: %w", err)
	}
	if err := t.html.Execute(&html, data); err != nil {
		return nil, fmt.Errorf("Mailer - Send - HTML: %w", err)
	}

	msg := gomail.NewMsg()
	if err := msg.From(m.from); err != nil {
		return nil, fmt.Errorf("Mailer - Send - msg.From: %w", err)
	}
	if err := msg.To(e.To); err != nil {
		return nil, fmt.Errorf("Mailer - Send - msg.To: %w", err)
	}
	msg.Subject(strings.TrimSpace(subject.String()))
	msg.SetDate()
	// The outbox ID makes a resend after a crash recognisable as the same message.
	msg.SetMessageIDWithValue(e.ID + "@" + m.domain)
	msg.SetBodyString(gomail.TypeTextPlain, text.String())
	msg.AddAlternativeString(gomail.TypeTextHTML, html.String())
	return msg, nil
}

// resetLink points at the web app's reset page. The token is in the fragment, which is never sent to
// a server, so it stays out of access logs and Referer headers. It is form-encoded, so the page can read
// it back with URLSearchParams.
func resetLink(appBaseURL, token string) string {
	return appBaseURL + "/reset-password#" + url.Values{"token": {token}}.Encode()
}

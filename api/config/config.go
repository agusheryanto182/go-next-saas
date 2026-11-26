// Package config loads the API configuration from environment variables and validates it at start-up,
// so a misconfigured deployment fails immediately instead of on the first request.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/netip"
	"net/url"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the complete API configuration. Variables are documented in .env.example.
type Config struct {
	HTTPAddr        string        `env:"HTTP_ADDR"        envDefault:":8080"`
	LogLevel        string        `env:"LOG_LEVEL"        envDefault:"info"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`

	// DatabaseURL connects as the runtime role, which is subject to row-level security (ADR 0008).
	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`
	// AppBaseURL is the public URL of the web app, used in invitation and reset links.
	AppBaseURL string `env:"APP_BASE_URL,required,notEmpty"`

	// TrustedProxies are the CIDRs whose X-Forwarded-For is believed when finding the client IP
	// (ADR 0006): the reverse proxy, and Next.js during server rendering. Comma-separated; empty
	// trusts nobody. Read them with TrustedProxyPrefixes.
	TrustedProxies []string `env:"TRUSTED_PROXIES"`

	// OTLPEndpoint enables trace export when set, e.g. http://localhost:4318.
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`

	SMTP SMTP `envPrefix:"SMTP_"`
	// MailFrom is the sender of every email, e.g. `go-next-saas <no-reply@example.com>`.
	MailFrom string `env:"MAIL_FROM,required,notEmpty"`
}

// SMTP is the outgoing mail server (ADR 0010). Mailpit locally.
type SMTP struct {
	Host     string `env:"HOST,required,notEmpty"`
	Port     int    `env:"PORT"     envDefault:"587"`
	Username string `env:"USERNAME"` // empty: no authentication
	Password string `env:"PASSWORD"`
	// TLS is none, starttls (production providers on 587) or tls (implicit TLS, usually 465).
	TLS            string        `env:"TLS"             envDefault:"starttls"`
	ConnectTimeout time.Duration `env:"CONNECT_TIMEOUT" envDefault:"10s"`
	SendTimeout    time.Duration `env:"SEND_TIMEOUT"    envDefault:"15s"`
}

const _maxSendTimeout = 20 * time.Second

// ErrInvalid is wrapped by every validation failure.
var ErrInvalid = errors.New("invalid value")

// New reads the environment and validates the result.
func New() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

// SlogLevel returns the parsed log level. validate guarantees it parses.
func (c Config) SlogLevel() slog.Level {
	var level slog.Level
	_ = level.UnmarshalText([]byte(c.LogLevel))
	return level
}

// TrustedProxyPrefixes returns the parsed TRUSTED_PROXIES. validate guarantees they parse.
func (c Config) TrustedProxyPrefixes() []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(c.TrustedProxies))
	for _, raw := range c.TrustedProxies {
		if p, err := netip.ParsePrefix(raw); err == nil {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

func (c Config) validate() error {
	var errs []error
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL %q: %w: use debug, info, warn or error", c.LogLevel, ErrInvalid))
	}
	if err := validateHTTPURL(c.AppBaseURL); err != nil {
		errs = append(errs, fmt.Errorf("APP_BASE_URL: %w", err))
	}
	if c.OTLPEndpoint != "" {
		if err := validateHTTPURL(c.OTLPEndpoint); err != nil {
			errs = append(errs, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT: %w", err))
		}
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w: must be positive", ErrInvalid))
	}
	for _, raw := range c.TrustedProxies {
		if err := validateCIDR(raw); err != nil {
			errs = append(errs, fmt.Errorf("TRUSTED_PROXIES: %w", err))
		}
	}
	errs = append(errs, c.SMTP.validate())
	if _, err := mail.ParseAddress(c.MailFrom); err != nil {
		errs = append(errs, fmt.Errorf("MAIL_FROM %q: %w: use an address like `Name <no-reply@example.com>`", c.MailFrom, ErrInvalid))
	}
	return errors.Join(errs...)
}

func (s SMTP) validate() error {
	var errs []error
	if s.Port < 1 || s.Port > 65535 {
		errs = append(errs, fmt.Errorf("SMTP_PORT %d: %w: must be between 1 and 65535", s.Port, ErrInvalid))
	}
	switch s.TLS {
	case "none", "starttls", "tls":
	default:
		errs = append(errs, fmt.Errorf("SMTP_TLS %q: %w: use none, starttls or tls", s.TLS, ErrInvalid))
	}
	if (s.Username == "") != (s.Password == "") {
		errs = append(errs, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD: %w: set both or neither", ErrInvalid))
	}
	if s.ConnectTimeout <= 0 {
		errs = append(errs, fmt.Errorf("SMTP_CONNECT_TIMEOUT: %w: must be positive", ErrInvalid))
	}
	// The outbox leases 10 emails for 5 minutes and sends them one by one (ADR 0010): each send must
	// fit a tenth of the lease with room for recording it, or a slow server causes duplicates.
	if s.SendTimeout <= 0 || s.SendTimeout > _maxSendTimeout {
		errs = append(errs, fmt.Errorf("SMTP_SEND_TIMEOUT %s: %w: must be positive and at most %s",
			s.SendTimeout, ErrInvalid, _maxSendTimeout))
	}
	return errors.Join(errs...)
}

// validateCIDR accepts a network in CIDR notation without host bits, like 10.0.0.0/8 or ::1/128.
func validateCIDR(raw string) error {
	p, err := netip.ParsePrefix(raw)
	if err != nil {
		return fmt.Errorf("%q: %w: use CIDR notation, like 10.0.0.0/8 or ::1/128", raw, ErrInvalid)
	}
	if p != p.Masked() {
		return fmt.Errorf("%q: %w: host bits are set, did you mean %s?", raw, ErrInvalid, p.Masked())
	}
	return nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%q: %w: must be an absolute http(s) URL", raw, ErrInvalid)
	}
	return nil
}

// Package config loads the API configuration from environment variables and validates it at start-up,
// so a misconfigured deployment fails immediately instead of on the first request.
package config

import (
	"errors"
	"fmt"
	"log/slog"
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

	// OTLPEndpoint enables trace export when set, e.g. http://localhost:4318.
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

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

func (c Config) validate() error {
	var errs []error
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL %q: use debug, info, warn or error", c.LogLevel))
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
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT must be positive"))
	}
	return errors.Join(errs...)
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%q must be an absolute http(s) URL", raw)
	}
	return nil
}

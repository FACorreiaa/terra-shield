// Package config loads the process-wide settings.
//
// Feature packages that need settings of their own declare a struct with
// `env` tags and call Load on it, so adding a variable never touches this
// file.
package config

import (
	"errors"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Environment is the deployment environment name.
type Environment string

const (
	Production  Environment = "production"
	Development Environment = "development"
)

// Base is what both processes (serve and worker) need.
type Base struct {
	Environment Environment `env:"APP_ENV" envDefault:"development"`
	Port        int         `env:"PORT" envDefault:"8080"`
	// MetricsPort is where the worker serves /metrics and /health/live.
	MetricsPort int    `env:"METRICS_PORT" envDefault:"9090"`
	GitSHA      string `env:"GIT_SHA"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat   string `env:"LOG_FORMAT" envDefault:"text"`

	DatabaseURL string `env:"DATABASE_URL"`

	// AllowedOrigins is a comma-separated CORS allow-list. Empty means no
	// CORS headers at all: the Apple clients do not need them, and the web
	// client (phase 2) sets this when it ships.
	AllowedOrigins string `env:"ALLOWED_ORIGINS"`
	// RateLimitRPS is the per-client-IP request rate the API allows.
	RateLimitRPS float64 `env:"RATE_LIMIT_RPS" envDefault:"20"`
	// RequestTimeout bounds every /v1 request except the live stream, so a
	// slow query cannot hold a connection (and a database slot) forever.
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"10s"`
	// AircraftFeedURL overrides the live flights feed (adsb.lol), so local
	// client work can run against a replay instead of the daily quota.
	AircraftFeedURL string `env:"AIRCRAFT_FEED_URL"`
	// WikidataEndpoint overrides the Wikidata Query Service for the bases
	// and historic layers, so local work can load from a mirror while the
	// service is rate-limiting. Ignored in production.
	WikidataEndpoint string `env:"WIKIDATA_ENDPOINT"`
}

// IsProduction reports whether this is the production environment.
func (b Base) IsProduction() bool { return b.Environment == Production }

// Version is the build's git SHA, or "unknown".
func (b Base) Version() string {
	if b.GitSHA != "" {
		return b.GitSHA
	}
	return "unknown"
}

// Origins parses AllowedOrigins. A wildcard is refused in production.
func (b Base) Origins() ([]string, error) {
	var out []string
	for _, o := range strings.Split(b.AllowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			if o == "*" && b.IsProduction() {
				return nil, errors.New("ALLOWED_ORIGINS must not be * in production")
			}
			out = append(out, o)
		}
	}
	return out, nil
}

// Load parses environment variables into a settings struct.
func Load[T any]() (T, error) {
	var cfg T
	err := env.Parse(&cfg)
	return cfg, err
}

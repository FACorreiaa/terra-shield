// Package sources serves GET /v1/sources: each feed's license, attribution
// and freshness (ADR 8, ADR 11).
package sources

import (
	"context"
	"net/http"
	"time"

	"shield/internal/domain"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Lister reads every source with its health. store.SourceRepository is one.
type Lister interface {
	List(ctx context.Context) ([]domain.SourceStatus, error)
}

// Feature mounts /v1/sources.
type Feature struct {
	sources Lister // nil when no database is configured (routes command)
	now     func() time.Time
}

// New builds the feature on a Lister.
func New(sources Lister) *Feature {
	return &Feature{sources: sources, now: time.Now}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "sources" }

// Source is the wire shape of one source.
type Source struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	License             string     `json:"license"`
	Attribution         string     `json:"attribution"`
	PollIntervalSeconds int        `json:"pollIntervalSeconds"`
	Enabled             bool       `json:"enabled"`
	Coverage            string     `json:"coverage,omitempty"`
	TermsURL            string     `json:"termsUrl,omitempty"`
	Cadence             string     `json:"cadence,omitempty"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	LastErrorAt         *time.Time `json:"lastErrorAt,omitempty"`
	Stale               bool       `json:"stale"`
}

type listResponse struct {
	Sources []Source `json:"sources"`
}

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/sources", f.list)
	return nil
}

func (f *Feature) list(c *echo.Context) error {
	if f.sources == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	rows, err := f.sources.List(c.Request().Context())
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=30")
	return httpx.OK(c, listResponse{Sources: toWire(rows, f.now())})
}

func toWire(rows []domain.SourceStatus, now time.Time) []Source {
	out := make([]Source, 0, len(rows))
	for i := range rows {
		out = append(out, FromStatus(&rows[i], now))
	}
	return out
}

// FromStatus is the wire shape of one source. Other features embed it
// (an event's sourceInfo), so every endpoint describes a source alike.
func FromStatus(r *domain.SourceStatus, now time.Time) Source {
	return Source{
		ID:                  r.ID,
		Name:                r.Name,
		URL:                 r.URL,
		License:             r.License,
		Attribution:         r.Attribution,
		PollIntervalSeconds: int(r.PollInterval / time.Second),
		Enabled:             r.Enabled,
		Coverage:            r.Scope,
		TermsURL:            r.TermsURL,
		Cadence:             r.Cadence,
		LastSuccessAt:       r.LastSuccessAt,
		LastErrorAt:         r.LastErrorAt,
		Stale:               r.Stale(now),
	}
}

// Package sources serves GET /v1/sources: each feed's license, attribution
// and freshness (ADR 8, ADR 11).
package sources

import (
	"time"

	"shield/internal/httpx"
	"shield/internal/server"
	"shield/internal/store/sourcesdb"

	"github.com/labstack/echo/v5"
)

// Feature mounts /v1/sources.
type Feature struct{}

// Name implements server.Feature.
func (Feature) Name() string { return "sources" }

// Source is the wire shape of one source.
type Source struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	License             string     `json:"license"`
	Attribution         string     `json:"attribution"`
	PollIntervalSeconds int32      `json:"pollIntervalSeconds"`
	Enabled             bool       `json:"enabled"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	LastErrorAt         *time.Time `json:"lastErrorAt,omitempty"`
	Stale               bool       `json:"stale"`
}

type listResponse struct {
	Sources []Source `json:"sources"`
}

// staleAfter is how many missed intervals make a source stale; it matches
// the staleness alert.
const staleAfter = 3

// Register implements server.Feature.
func (Feature) Register(d *server.Deps, r *server.Routes) error {
	r.V1.GET("/sources", func(c *echo.Context) error {
		if d.DB == nil {
			return httpx.Abort(503, "database not configured")
		}
		rows, err := sourcesdb.New(d.DB).ListSources(c.Request().Context())
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "public, max-age=30")
		return httpx.OK(c, listResponse{Sources: toWire(rows, time.Now())})
	})
	return nil
}

func toWire(rows []sourcesdb.ListSourcesRow, now time.Time) []Source {
	out := make([]Source, 0, len(rows))
	for _, r := range rows {
		interval := time.Duration(r.PollIntervalSeconds) * time.Second
		out = append(out, Source{
			ID:                  r.ID,
			Name:                r.Name,
			URL:                 r.Url,
			License:             r.License,
			Attribution:         r.Attribution,
			PollIntervalSeconds: r.PollIntervalSeconds,
			Enabled:             r.Enabled,
			LastSuccessAt:       r.LastSuccessAt,
			LastErrorAt:         r.LastErrorAt,
			Stale:               r.Enabled && (r.LastSuccessAt == nil || now.Sub(*r.LastSuccessAt) > staleAfter*interval),
		})
	}
	return out
}

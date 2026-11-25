// Package bases serves the military bases layer (L3, ADR 29): GET
// /v1/bases. Static reference data from Wikidata, not events.
package bases

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"shield/internal/domain"
	"shield/internal/features/params"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Repository reads the layer. store.BaseRepository is one.
type Repository interface {
	InBox(ctx context.Context, bb *domain.BBox) ([]domain.MilitaryBase, error)
}

// maxAge: the layer is refreshed weekly.
const maxAge = time.Hour

// Feature mounts /v1/bases.
type Feature struct {
	bases Repository // nil when no database is configured (routes command)
}

// New builds the feature.
func New(r Repository) *Feature { return &Feature{bases: r} }

// Name implements server.Feature.
func (*Feature) Name() string { return "bases" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/bases", f.list)
	return nil
}

// Base is one installation on the wire.
type Base struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Kind        string  `json:"kind"`
	Class       string  `json:"class"`
	Operator    string  `json:"operator,omitempty"`
	Country     string  `json:"country,omitempty"`
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	URL         string  `json:"url"`
}

type listResponse struct {
	Bases       []Base     `json:"bases"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	Attribution string     `json:"attribution"`
	License     string     `json:"license"`
	Note        string     `json:"note"`
}

const note = "Installations listed on Wikidata with coordinates and no recorded closure. Wikidata records closures unevenly, so a listing is not proof a base is in use."

func (f *Feature) list(c *echo.Context) error {
	if f.bases == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	bb, err := params.BBox(c.Request().URL.Query())
	if err != nil {
		return err
	}
	rows, err := f.bases.InBox(c.Request().Context(), bb)
	if err != nil {
		return err
	}
	out := listResponse{Bases: make([]Base, 0, len(rows)), Attribution: "Wikidata", License: "CC0 1.0", Note: note}
	for i := range rows {
		b := &rows[i]
		out.Bases = append(out.Bases, Base{
			ID: b.QID, Name: b.Name, Description: b.Description, Kind: string(b.Kind), Class: b.Class,
			Operator: b.Operator, Country: b.ISO2, Lon: b.Lon, Lat: b.Lat,
			URL: "https://www.wikidata.org/wiki/" + b.QID,
		})
		if out.UpdatedAt == nil || b.UpdatedAt.After(*out.UpdatedAt) {
			at := b.UpdatedAt.UTC()
			out.UpdatedAt = &at
		}
	}
	body, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return httpx.Cached(c, echo.MIMEApplicationJSON, body, httpx.ETag(body), maxAge)
}

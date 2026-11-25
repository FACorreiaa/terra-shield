// Package historic serves the historic layer (L4, ADR 30): GET
// /v1/historic, forts, castles, bunkers, war cemeteries and other historic
// sites from Wikidata, as GeoJSON: a grid of counts when zoomed out and
// single sites when zoomed in, like the fire layer. Static reference
// data, not events.
package historic

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"shield/internal/domain"
	"shield/internal/features/params"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Repository reads the layer. store.HistoricRepository is one.
type Repository interface {
	Grid(ctx context.Context, q domain.HistoricQuery, cellDegrees float64) ([]domain.HistoricCell, error)
	Points(ctx context.Context, q domain.HistoricQuery, limit int) ([]domain.HistoricSite, bool, error)
}

const (
	// pointsFromZoom: below it the layer is a grid, from it single sites.
	// Measured 2026-11-25 on 53,265 sites: the densest zoom-7 view (central
	// Europe) holds 13,334 sites, past maxPoints; the densest zoom-8 view
	// 6,957 (230 KB gzipped).
	pointsFromZoom = 8
	// gridBBoxFromZoom: from it the grid also needs a bbox. The whole world
	// is 8,238 cells (56 KB gzipped) at zoom 5 but 25,913 at zoom 7.
	gridBBoxFromZoom = 6
	// maxPoints bounds a site view; a busier view is marked truncated.
	maxPoints = 10_000
	// maxAge: the layer is refreshed weekly.
	maxAge      = time.Hour
	mimeGeoJSON = "application/geo+json"
	// note: some barracks and castles here are still in use; the layer is
	// a heritage view, not a list of what is or is not active.
	note = "Historic sites listed on Wikidata: forts, castles, bunkers, barracks, war cemeteries and former bases. Some are ruins or memorials and some are still in use; for military installations listed as in use, see the bases layer."
)

// Feature mounts /v1/historic.
type Feature struct {
	sites Repository // nil when no database is configured (routes command)
}

// New builds the feature.
func New(r Repository) *Feature { return &Feature{sites: r} }

// Name implements server.Feature.
func (*Feature) Name() string { return "historic" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/historic", f.list)
	return nil
}

func (f *Feature) list(c *echo.Context) error {
	if f.sites == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	v := c.Request().URL.Query()
	bbox, err := params.BBox(v)
	if err != nil {
		return err
	}
	zoom, err := params.Zoom(v)
	if err != nil {
		return err
	}
	kinds, err := params.List(v, "kind", domain.ParseHistoricKind)
	if err != nil {
		return err
	}
	q := domain.HistoricQuery{BBox: bbox, Kinds: kinds}

	ctx := c.Request().Context()
	fc := collection{Type: "FeatureCollection", Attribution: "Wikidata", License: "CC0 1.0", Note: note}
	if zoom == nil || *zoom < pointsFromZoom {
		z := 0
		if zoom != nil {
			z = *zoom
		}
		if bbox == nil && z >= gridBBoxFromZoom {
			return httpx.BadRequest("bbox is required from zoom 6")
		}
		cell := CellDegrees(z)
		cells, err := f.sites.Grid(ctx, q, cell)
		if err != nil {
			return err
		}
		fc.Aggregation, fc.CellDegrees = "grid", cell
		fc.Features = make([]feature, 0, len(cells))
		for i := range cells {
			cl := &cells[i]
			ks := make(map[string]int, len(cl.Kinds))
			for k, n := range cl.Kinds {
				ks[string(k)] = n
			}
			fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: at(cl.Lon, cl.Lat), Properties: Cell{Count: cl.Count, Kinds: ks}})
		}
	} else {
		if bbox == nil {
			return httpx.BadRequest("bbox is required from zoom 8, where sites are single points")
		}
		pts, truncated, err := f.sites.Points(ctx, q, maxPoints)
		if err != nil {
			return err
		}
		fc.Aggregation, fc.Truncated = "none", truncated
		fc.Features = make([]feature, 0, len(pts))
		for i := range pts {
			s := &pts[i]
			fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: at(s.Lon, s.Lat), Properties: Site{
				ID: s.QID, Name: s.Name, Description: s.Description, Kind: string(s.Kind), Class: s.Class,
				Country: s.ISO2, URL: "https://www.wikidata.org/wiki/" + s.QID,
			}})
			if fc.UpdatedAt == nil || s.UpdatedAt.After(*fc.UpdatedAt) {
				u := s.UpdatedAt.UTC()
				fc.UpdatedAt = &u
			}
		}
	}
	body, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeGeoJSON, body, httpx.ETag(body), maxAge)
}

// CellDegrees is the grid at zoom: about 8 screen pixels, as for fires.
func CellDegrees(zoom int) float64 {
	return 360 / math.Exp2(float64(zoom)+5)
}

type point struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type feature struct {
	Type       string `json:"type"`
	Geometry   point  `json:"geometry"`
	Properties any    `json:"properties"`
}

type collection struct {
	Type        string     `json:"type"`
	Aggregation string     `json:"aggregation"`
	CellDegrees float64    `json:"cellDegrees,omitempty"`
	Features    []feature  `json:"features"`
	Truncated   bool       `json:"truncated"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	Attribution string     `json:"attribution"`
	License     string     `json:"license"`
	Note        string     `json:"note"`
}

// Site is one historic site.
type Site struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	Class       string `json:"class"`
	Country     string `json:"country,omitempty"`
	URL         string `json:"url"`
}

// Cell is the sites in one grid cell, in total and per kind.
type Cell struct {
	Count int            `json:"count"`
	Kinds map[string]int `json:"kinds"`
}

// round5 keeps 5 decimals (about 1 m).
func round5(v float64) float64 { return math.Round(v*1e5) / 1e5 }

func at(lon, lat float64) point {
	return point{Type: "Point", Coordinates: [2]float64{round5(lon), round5(lat)}}
}

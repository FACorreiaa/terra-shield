// Package fires serves the satellite fire layer (S2.5): NASA FIRMS
// detections as GeoJSON, a heatmap grid when zoomed out and single points
// when zoomed in (ADR 10). Detections are thermal anomalies, never
// "wildfires".
package fires

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

// Repository reads detections. store.FireRepository is one.
type Repository interface {
	// Grid aggregates detections into cells of cellDegrees.
	Grid(ctx context.Context, q domain.FireQuery, cellDegrees float64) ([]domain.FireCell, error)
	// Points returns up to limit detections and whether more matched.
	Points(ctx context.Context, q domain.FireQuery, limit int) ([]domain.FireDetection, bool, error)
}

const (
	// pointsFromZoom: below it the layer is a grid, from it single points.
	pointsFromZoom = 6
	// maxPoints bounds a point view; a busier view is marked truncated.
	maxPoints = 25_000
	// defaultWindow and maxWindow bound `since`; the table keeps 48 h.
	defaultWindow = 24 * time.Hour
	maxWindow     = 48 * time.Hour
	// maxAge: FIRMS updates every 15 min at most, so a minute of caching
	// costs nothing in freshness.
	maxAge      = time.Minute
	mimeGeoJSON = "application/geo+json"
)

// Feature mounts /v1/fires.
type Feature struct {
	fires Repository // nil when no database is configured (routes command)
	now   func() time.Time
}

// New builds the feature on a Repository.
func New(fires Repository) *Feature {
	return &Feature{fires: fires, now: time.Now}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "fires" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/fires", f.list)
	return nil
}

func (f *Feature) list(c *echo.Context) error {
	if f.fires == nil {
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
	since, err := params.Time(v, "since")
	if err != nil {
		return err
	}
	now := f.now().UTC()
	q := domain.FireQuery{BBox: bbox, Since: now.Add(-defaultWindow)}
	if since != nil {
		// Older than the window would silently return less than asked;
		// clamping keeps the answer honest about what exists.
		q.Since = maxTime(*since, now.Add(-maxWindow))
	}

	ctx := c.Request().Context()
	var fc collection
	if zoom == nil || *zoom < pointsFromZoom {
		z := 0
		if zoom != nil {
			z = *zoom
		}
		cell := CellDegrees(z)
		cells, err := f.fires.Grid(ctx, q, cell)
		if err != nil {
			return err
		}
		fc = gridCollection(cells, cell)
	} else {
		if bbox == nil {
			return httpx.BadRequest("bbox is required from zoom 6, where detections are single points")
		}
		pts, truncated, err := f.fires.Points(ctx, q, maxPoints)
		if err != nil {
			return err
		}
		fc = pointCollection(pts, truncated)
	}
	body, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeGeoJSON, body, httpx.ETag(body), maxAge)
}

// CellDegrees is the heatmap grid at zoom: about 8 screen pixels, so 11.25°
// at zoom 0 and 0.35° at zoom 5.
func CellDegrees(zoom int) float64 {
	return 360 / math.Exp2(float64(zoom)+5)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
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
	Type        string    `json:"type"`
	Aggregation string    `json:"aggregation"`
	CellDegrees float64   `json:"cellDegrees,omitempty"`
	Features    []feature `json:"features"`
	Truncated   bool      `json:"truncated"`
}

// Detection is one VIIRS detection.
type Detection struct {
	Satellite  string    `json:"satellite"`
	AcquiredAt time.Time `json:"acquiredAt"`
	FRP        *float32  `json:"frp,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
	ISO2       string    `json:"iso2,omitempty"`
}

// Cell is the detections in one grid cell.
type Cell struct {
	Count    int       `json:"count"`
	MaxFRP   *float32  `json:"maxFrp,omitempty"`
	LatestAt time.Time `json:"latestAt"`
}

// round5 keeps 5 decimals (about 1 m): VIIRS pixels are 375 m.
func round5(v float64) float64 { return math.Round(v*1e5) / 1e5 }

func at(lon, lat float64) point {
	return point{Type: "Point", Coordinates: [2]float64{round5(lon), round5(lat)}}
}

func gridCollection(cells []domain.FireCell, cellDegrees float64) collection {
	fc := collection{Type: "FeatureCollection", Aggregation: "grid", CellDegrees: cellDegrees, Features: make([]feature, 0, len(cells))}
	for _, c := range cells {
		fc.Features = append(fc.Features, feature{
			Type: "Feature", Geometry: at(c.Lon, c.Lat),
			Properties: Cell{Count: c.Count, MaxFRP: c.MaxFRP, LatestAt: c.LatestAt.UTC()},
		})
	}
	return fc
}

func pointCollection(pts []domain.FireDetection, truncated bool) collection {
	fc := collection{Type: "FeatureCollection", Aggregation: "none", Features: make([]feature, 0, len(pts)), Truncated: truncated}
	for _, d := range pts {
		fc.Features = append(fc.Features, feature{
			Type: "Feature", Geometry: at(d.Lon, d.Lat),
			Properties: Detection{Satellite: d.Satellite, AcquiredAt: d.AcquiredAt.UTC(), FRP: d.FRP, Confidence: d.Confidence, ISO2: d.ISO2},
		})
	}
	return fc
}

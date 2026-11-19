// Package aircraft serves the live flights layer (L1): GET /v1/aircraft.
// Positions come from adsb.lol through a per-cell cache (internal/aircraft)
// and are never stored.
package aircraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"shield/internal/aircraft"
	"shield/internal/domain"
	"shield/internal/features/params"
	"shield/internal/httpx"
	"shield/internal/server"
	"shield/internal/sources/adsblol"

	"github.com/labstack/echo/v5"
)

// Finder returns the aircraft in a bbox. *aircraft.Service is one.
type Finder interface {
	InBox(ctx context.Context, bb domain.BBox) (aircraft.Result, error)
	MaxCells() int
}

// maxAge: the cache refreshes a cell at most once a minute, so clients
// polling faster only get the same answer.
const maxAge = 15 * time.Second

// Feature mounts /v1/aircraft.
type Feature struct {
	finder Finder // nil when not configured (routes command)
}

// New builds the feature.
func New(f Finder) *Feature { return &Feature{finder: f} }

// Name implements server.Feature.
func (*Feature) Name() string { return "aircraft" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/aircraft", f.list)
	return nil
}

// Aircraft is one position on the wire.
type Aircraft struct {
	Hex             string    `json:"hex"`
	Callsign        string    `json:"callsign,omitempty"`
	Registration    string    `json:"registration,omitempty"`
	TypeCode        string    `json:"typeCode,omitempty"`
	Lon             float64   `json:"lon"`
	Lat             float64   `json:"lat"`
	AltitudeFt      *int      `json:"altitudeFt,omitempty"`
	OnGround        bool      `json:"onGround"`
	GroundSpeedKt   *float64  `json:"groundSpeedKt,omitempty"`
	TrackDeg        *float64  `json:"trackDeg,omitempty"`
	VerticalRateFpm *int      `json:"verticalRateFpm,omitempty"`
	Squawk          string    `json:"squawk,omitempty"`
	Emergency       string    `json:"emergency,omitempty"`
	Military        bool      `json:"military"`
	PositionAt      time.Time `json:"positionAt"`
}

type listResponse struct {
	Aircraft    []Aircraft `json:"aircraft"`
	DataAt      *time.Time `json:"dataAt,omitempty"`
	Complete    bool       `json:"complete"`
	Attribution string     `json:"attribution"`
	License     string     `json:"license"`
	LicenseURL  string     `json:"licenseUrl"`
}

func (f *Feature) list(c *echo.Context) error {
	if f.finder == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "live flights not configured")
	}
	bb, err := params.BBox(c.Request().URL.Query())
	if err != nil {
		return err
	}
	if bb == nil {
		return httpx.BadRequest("bbox is required: live flights are served for a map view, never worldwide")
	}
	res, err := f.finder.InBox(c.Request().Context(), *bb)
	if errors.Is(err, aircraft.ErrTooLarge) {
		return httpx.BadRequest(fmt.Sprintf("bbox spans more than %d cells of %g°: zoom in", f.finder.MaxCells(), aircraft.CellDegrees))
	}
	if err != nil {
		return err
	}
	out := listResponse{
		Aircraft:    make([]Aircraft, 0, len(res.Aircraft)),
		Complete:    res.Complete,
		Attribution: adsblol.Attribution,
		License:     adsblol.License,
		LicenseURL:  adsblol.LicenseURL,
	}
	if !res.DataAt.IsZero() {
		out.DataAt = &res.DataAt
	}
	for i := range res.Aircraft {
		out.Aircraft = append(out.Aircraft, toWire(&res.Aircraft[i]))
	}
	body, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return httpx.Cached(c, echo.MIMEApplicationJSON, body, httpx.ETag(body), maxAge)
}

func toWire(a *domain.Aircraft) Aircraft {
	return Aircraft{
		Hex: a.Hex, Callsign: a.Callsign, Registration: a.Registration, TypeCode: a.TypeCode,
		Lon: a.Lon, Lat: a.Lat, AltitudeFt: a.AltitudeFt, OnGround: a.OnGround,
		GroundSpeedKt: a.GroundSpeedKt, TrackDeg: a.TrackDeg, VerticalRateFpm: a.VerticalRateFpm,
		Squawk: a.Squawk, Emergency: a.Emergency, Military: a.Military, PositionAt: a.PositionAt.UTC(),
	}
}

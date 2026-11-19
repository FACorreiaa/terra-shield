// Package adsblol reads live aircraft positions from adsb.lol (L1).
//
// Data are ODbL 1.0: attribution "© adsb.lol contributors", share-alike
// on any database published from them; Shield publishes none (positions
// are cached for seconds, never stored). Without a key the API allows
// 500 requests a day per IP (owner, 2026-11-19: no key for now), so the
// aircraft service, not this client, decides when to call.
package adsblol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

const (
	// BaseURL is the public API. AIRCRAFT_FEED_URL overrides it, so
	// client work can run against a local replay instead of the quota.
	BaseURL = "https://api.adsb.lol"
	// MaxRadiusNM is the largest circle /v2/point accepts, in nautical
	// miles (measured: radius 120 returned aircraft up to 116.9 NM out).
	MaxRadiusNM = 250
	// MaxPositionAge drops aircraft whose last position is older: the
	// feed itself moves those to lastPosition after 60 s.
	MaxPositionAge = 60 * time.Second

	Attribution = "© adsb.lol contributors"
	License     = "ODbL 1.0"
	LicenseURL  = "https://opendatacommons.org/licenses/odbl/1-0/"

	// dbFlags bits (adsb.lol API reference).
	flagMilitary = 1
	flagPIA      = 4 // privacy ICAO address: the operator asked for anonymity
	flagLADD     = 8 // FAA Limiting Aircraft Data Displayed: asked not to be shown
)

// ID is the source's id in /v1/sources.
const ID = "adsblol"

// Info describes the feed for /v1/sources. The API registers it at start:
// the worker never polls it (OnDemand), and its aircraft are not hazards,
// so it never counts as a country's coverage.
func Info(refresh time.Duration) domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "adsb.lol",
		URL:          "https://adsb.lol/",
		License:      License,
		Attribution:  Attribution,
		PollInterval: refresh,
		Enabled:      true,
		NonHazard:    true,
		OnDemand:     true,
		Scope:        "Live aircraft positions worldwide, for map views up to 10° across",
		TermsURL:     "https://www.adsb.lol/docs/open-data/api/",
		Cadence:      "continuously; fetched while someone views the flights layer",
	}
}

// Client queries one circle at a time. Safe for concurrent use.
type Client struct {
	base string
	http *fetch.Client
}

// New builds a Client on base (BaseURL when empty).
func New(c *fetch.Client, base string) *Client {
	if base == "" {
		base = BaseURL
	}
	return &Client{base: strings.TrimRight(base, "/"), http: c}
}

// Area returns the aircraft within radiusNM of (lat, lon). A 429 is a
// *domain.QuotaError.
func (c *Client) Area(ctx context.Context, lat, lon float64, radiusNM int) (domain.AircraftArea, error) {
	if radiusNM < 1 || radiusNM > MaxRadiusNM {
		return domain.AircraftArea{}, fmt.Errorf("radius %d NM outside 1–%d", radiusNM, MaxRadiusNM)
	}
	url := fmt.Sprintf("%s/v2/point/%.4f/%.4f/%d", c.base, lat, lon, radiusNM)
	resp, err := c.http.Get(ctx, url, fetch.Validators{})
	if err != nil {
		var se *fetch.StatusError
		if errors.As(err, &se) && se.Status == http.StatusTooManyRequests {
			return domain.AircraftArea{}, &domain.QuotaError{RetryAfter: se.RetryAfter}
		}
		return domain.AircraftArea{}, err
	}
	area, err := Parse(resp.Body)
	if err != nil {
		return domain.AircraftArea{}, err
	}
	area.QuotaRemaining = remaining(resp.Header)
	return area, nil
}

// remaining reads x-ratelimit-remaining; -1 when absent or malformed.
func remaining(h http.Header) int {
	n, err := strconv.Atoi(strings.TrimSpace(h.Get("X-Ratelimit-Remaining")))
	if err != nil || n < 0 {
		return -1
	}
	return n
}

type envelope struct {
	AC  *[]json.RawMessage `json:"ac"`
	Now int64              `json:"now"` // Unix milliseconds
	Msg string             `json:"msg"`
}

type rawAircraft struct {
	Hex       string          `json:"hex"`
	Flight    string          `json:"flight"`
	R         string          `json:"r"`
	T         string          `json:"t"`
	Lat       *float64        `json:"lat"`
	Lon       *float64        `json:"lon"`
	AltBaro   json.RawMessage `json:"alt_baro"`
	GS        *float64        `json:"gs"`
	Track     *float64        `json:"track"`
	BaroRate  *int            `json:"baro_rate"`
	Squawk    string          `json:"squawk"`
	Emergency string          `json:"emergency"`
	DBFlags   int             `json:"dbFlags"`
	SeenPos   *float64        `json:"seen_pos"`
}

// Parse reads a /v2 response. A wrong envelope fails; an aircraft that
// does not decode, has no current position, or whose operator opted out of
// display (LADD, PIA) is left out, so one odd entry never costs the whole
// answer (and the quota it took).
func Parse(body []byte) (domain.AircraftArea, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.AircraftArea{}, fmt.Errorf("adsb.lol: %w", err)
	}
	if env.AC == nil || env.Now <= 0 {
		return domain.AircraftArea{}, fmt.Errorf("adsb.lol: response without ac or now (msg %q)", env.Msg)
	}
	at := time.UnixMilli(env.Now).UTC()
	out := make([]domain.Aircraft, 0, len(*env.AC))
	for _, raw := range *env.AC {
		var r rawAircraft
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		if a, ok := convert(&r, at); ok {
			out = append(out, a)
		}
	}
	return domain.AircraftArea{Aircraft: out, At: at, QuotaRemaining: -1}, nil
}

func convert(r *rawAircraft, at time.Time) (domain.Aircraft, bool) {
	if r.Hex == "" || r.Lat == nil || r.Lon == nil || r.SeenPos == nil ||
		*r.Lat < -90 || *r.Lat > 90 || *r.Lon < -180 || *r.Lon > 180 {
		return domain.Aircraft{}, false
	}
	if r.DBFlags&(flagLADD|flagPIA) != 0 {
		return domain.Aircraft{}, false
	}
	age := time.Duration(*r.SeenPos * float64(time.Second))
	if age < 0 || age > MaxPositionAge {
		return domain.Aircraft{}, false
	}
	a := domain.Aircraft{
		Hex:             strings.ToLower(r.Hex),
		Callsign:        strings.TrimSpace(r.Flight),
		Registration:    r.R,
		TypeCode:        r.T,
		Lon:             *r.Lon,
		Lat:             *r.Lat,
		GroundSpeedKt:   r.GS,
		TrackDeg:        r.Track,
		VerticalRateFpm: r.BaroRate,
		Squawk:          r.Squawk,
		Military:        r.DBFlags&flagMilitary != 0,
		PositionAt:      at.Add(-age).Truncate(time.Second),
	}
	if r.Emergency != "" && r.Emergency != "none" {
		a.Emergency = r.Emergency
	}
	var alt int
	var s string
	switch {
	case json.Unmarshal(r.AltBaro, &alt) == nil:
		a.AltitudeFt = &alt
	case json.Unmarshal(r.AltBaro, &s) == nil && s == "ground":
		a.OnGround = true
	}
	return a, true
}

// Package params parses the query parameters several features share
// (map viewport, zoom, time bounds), so every endpoint applies the same
// rules and error messages.
package params

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/httpx"
)

// MaxZoom is the deepest map zoom accepted.
const MaxZoom = 22

// BBox reads "bbox" (minLon,minLat,maxLon,maxLat, WGS 84); nil when absent.
// MinLon greater than MaxLon crosses the antimeridian.
func BBox(v url.Values) (*domain.BBox, error) {
	s := v.Get("bbox")
	if s == "" {
		return nil, nil
	}
	bad := httpx.BadRequest("bbox must be minLon,minLat,maxLon,maxLat in degrees")
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, bad
	}
	var f [4]float64
	for i, part := range parts {
		x, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, bad
		}
		f[i] = x
	}
	bb := &domain.BBox{MinLon: f[0], MinLat: f[1], MaxLon: f[2], MaxLat: f[3]}
	if bb.MinLon < -180 || bb.MinLon > 180 || bb.MaxLon < -180 || bb.MaxLon > 180 ||
		bb.MinLat < -90 || bb.MaxLat > 90 || bb.MinLat > bb.MaxLat {
		return nil, bad
	}
	return bb, nil
}

// Radius bounds for "near": a city to a region. Wider questions use bbox
// or country.
const (
	DefaultRadiusKm = 100
	MaxRadiusKm     = 1000
)

// Near reads "near" (lon,lat in WGS 84, the same order as bbox) and
// "radiusKm" (1–1000, default 100); nil when near is absent. radiusKm
// without near is an error, never silently ignored.
func Near(v url.Values) (*domain.Near, error) {
	s, r := v.Get("near"), v.Get("radiusKm")
	if s == "" {
		if r != "" {
			return nil, httpx.BadRequest("radiusKm needs near")
		}
		return nil, nil
	}
	parts := strings.Split(s, ",")
	bad := httpx.BadRequest("near must be lon,lat in degrees")
	if len(parts) != 2 {
		return nil, bad
	}
	lon, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return nil, bad
	}
	n := &domain.Near{Lon: lon, Lat: lat, RadiusKm: DefaultRadiusKm}
	if r != "" {
		km, err := strconv.ParseFloat(r, 64)
		if err != nil || km < 1 || km > MaxRadiusKm {
			return nil, httpx.BadRequest("radiusKm must be a number from 1 to 1000")
		}
		n.RadiusKm = km
	}
	return n, nil
}

// Zoom reads "zoom" (0–22); nil when absent.
func Zoom(v url.Values) (*int, error) {
	s := v.Get("zoom")
	if s == "" {
		return nil, nil
	}
	z, err := strconv.Atoi(s)
	if err != nil || z < 0 || z > MaxZoom {
		return nil, httpx.BadRequest("zoom must be an integer from 0 to 22")
	}
	return &z, nil
}

// Time reads an RFC 3339 parameter as UTC; nil when absent.
func Time(v url.Values, name string) (*time.Time, error) {
	s := v.Get(name)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, httpx.BadRequest(name + " must be an RFC 3339 date-time")
	}
	t = t.UTC()
	return &t, nil
}

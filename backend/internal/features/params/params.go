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

package domain

import "encoding/json"

// Country is one admin-0 boundary to load (ADR 4).
type Country struct {
	ISO2, ISO3, Name string
	Geometry         json.RawMessage // GeoJSON geometry, WGS84
}

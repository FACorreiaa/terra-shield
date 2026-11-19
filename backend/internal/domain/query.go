package domain

import (
	"errors"
	"time"
)

// Relation is how an event relates to a country (ADR 4).
type Relation string

const (
	// RelationWithin: the event's geometry intersects the country.
	RelationWithin Relation = "within"
	// RelationNearby: offshore, within 250 km of the country.
	RelationNearby Relation = "nearby"
)

// EventCountry is one country an event touches.
type EventCountry struct {
	ISO2       string   `json:"iso2"`
	Relation   Relation `json:"relation"`
	DistanceKm float64  `json:"distanceKm"`
}

// EventRecord is a stored event as read back: the normalized Event plus
// what the store adds. Geometry is set only when the query asked for it.
type EventRecord struct {
	Event
	ID        string
	SourceID  string
	Latitude  float64 // centroid
	Longitude float64
	FetchedAt time.Time
	Countries []EventCountry
	// DistanceKm, with EventQuery.Near, is the distance from the point to
	// the event's geometry: 0 inside a warning area or footprint.
	DistanceKm *float64
}

// Near selects events within RadiusKm of a point, nearest first.
type Near struct {
	Lon, Lat, RadiusKm float64
}

// BBox is a lon/lat rectangle in WGS 84. MinLon > MaxLon crosses the
// antimeridian.
type BBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// EventQuery filters events. Empty slices and nil pointers mean "any".
type EventQuery struct {
	BBox       *BBox
	Types      []EventType
	Severities []string
	Kinds      []Kind
	Sources    []string
	Statuses   []Status
	Country    string // ISO2
	Near       *Near
	Since      *time.Time
	Until      *time.Time
	Limit      int

	// Geometry asks for each event's GeoJSON geometry, simplified with
	// tolerance SimplifyDegrees (0 keeps it exact).
	Geometry        bool
	SimplifyDegrees float64
}

// ErrNotFound is what repositories return for a missing record.
var ErrNotFound = errors.New("not found")

// Revision is one recorded change of an event (ADR 6): changed fields as
// {"from", "to"}, and {"geometry": {"changed": true}} for a new shape.
type Revision struct {
	At      time.Time
	Changes map[string]any
}

// EventDetail is one event with its exact geometry, its source and its
// newest revisions.
type EventDetail struct {
	EventRecord
	Source    SourceStatus
	Revisions []Revision
}

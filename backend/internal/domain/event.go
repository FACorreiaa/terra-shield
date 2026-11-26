package domain

import (
	"encoding/json"
	"time"
)

// EventType is what happened: a hazard, or an armed conflict (ADR 27).
// Values match the events.type check constraint.
type EventType string

// Event types.
const (
	TypeEarthquake      EventType = "earthquake"
	TypeWildfire        EventType = "wildfire"
	TypeFlood           EventType = "flood"
	TypeStorm           EventType = "storm"
	TypeTropicalCyclone EventType = "tropical_cyclone"
	TypeVolcano         EventType = "volcano"
	TypeDrought         EventType = "drought"
	TypeHeat            EventType = "heat"
	TypeCold            EventType = "cold"
	TypeHeavyRain       EventType = "heavy_rain"
	TypeWind            EventType = "wind"
	TypeSnowIce         EventType = "snow_ice"
	TypeTsunami         EventType = "tsunami"
	TypeOther           EventType = "other"
	TypeConflict        EventType = "conflict"
)

// Kind says what sort of statement a record is (ADR 3).
type Kind string

// Kinds.
const (
	KindObserved  Kind = "observed"
	KindWarning   Kind = "warning"
	KindForecast  Kind = "forecast"
	KindDetection Kind = "detection"
)

// Status is an event's lifecycle state.
type Status string

// Statuses.
const (
	StatusActive   Status = "active"
	StatusResolved Status = "resolved"
)

// Event is one normalized record from a source. Adapters fill every field
// from their feed; the store owns ids, countries and fetch timestamps.
type Event struct {
	SourceEventID  string
	Type           EventType
	Kind           Kind
	Status         Status
	Severity       string // derived, CAP-aligned (ADR 2): severity.Level.String()
	SourceSeverity string // the source's own value, verbatim; "" for none
	Title          string
	Summary        string
	Geometry       json.RawMessage // GeoJSON geometry, WGS84; Z is dropped
	StartedAt      *time.Time
	ObservedAt     time.Time
	UpdatedAt      time.Time // the source's own last-update time
	EndedAt        *time.Time
	URL            string // official page for the event (ADR 12)
	Attrs          map[string]any
	CountryRule    CountryRule // zero: every country the geometry touches
}

// CountryRule narrows country assignment (ADR 4) for an event whose area
// a national service drew: Issuer is always assigned, and another country
// only when it holds at least MinShare of the event's area.
//
// Listed, when set, replaces geometry altogether: the source names the
// countries an event applies to (a tsunami threat forecast lists the
// coasts it expects waves on, far from the marker at the epicenter).
// Codes not in the countries table are ignored.
type CountryRule struct {
	Issuer   string   // ISO 3166-1 alpha-2
	MinShare float64  // 0..1
	Listed   []string // ISO 3166-1 alpha-2
}

// PointGeometry is a GeoJSON Point.
func PointGeometry(lon, lat float64) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": [2]float64{lon, lat}}) //nolint:errchkjson // two floats cannot fail
	return b
}

// Valid reports whether t is one of the types above (the DB check).
func (t EventType) Valid() bool {
	switch t {
	case TypeEarthquake, TypeWildfire, TypeFlood, TypeStorm, TypeTropicalCyclone, TypeVolcano, TypeDrought,
		TypeHeat, TypeCold, TypeHeavyRain, TypeWind, TypeSnowIce, TypeTsunami, TypeOther, TypeConflict:
		return true
	}
	return false
}

// Valid reports whether k is one of the kinds above.
func (k Kind) Valid() bool {
	switch k {
	case KindObserved, KindWarning, KindForecast, KindDetection:
		return true
	}
	return false
}

// Valid reports whether s is one of the statuses above.
func (s Status) Valid() bool { return s == StatusActive || s == StatusResolved }

// ZoneRef names one NWS zone of a set (nwszones.Set.Kind): forecast,
// fire or county.
type ZoneRef struct {
	Set, UGC string
}

// ForecastZone is one NWS zone (UGC code like HIZ001, or NCC037 for a
// county): its rings as a GeoJSON MultiLineString, which the store
// assembles into polygons.
type ForecastZone struct {
	UGC   string
	State string
	Name  string
	Rings json.RawMessage
}

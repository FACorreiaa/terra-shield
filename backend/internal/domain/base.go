package domain

import "time"

// BaseKind groups Wikidata's installation classes for map glyphs.
type BaseKind string

const (
	BaseAir     BaseKind = "air"
	BaseNaval   BaseKind = "naval"
	BaseMissile BaseKind = "missile"
	BaseRadar   BaseKind = "radar"
	BaseGeneral BaseKind = "base"
)

// MilitaryBase is one installation on the bases layer (L3, ADR 29): a
// Wikidata item with coordinates and no recorded closure. It is static
// reference data, not an event: no time, no severity.
type MilitaryBase struct {
	QID         string // Wikidata item, e.g. "Q161348"
	Name        string
	Description string
	Kind        BaseKind
	Class       string // Wikidata class, in English ("airbase")
	Operator    string // who runs it (P137), when listed
	Lon, Lat    float64
	// ISO2 is where it sits, from Shield's country geometry, else
	// Wikidata's country (P17); empty at sea or when unknown.
	ISO2      string
	UpdatedAt time.Time
}

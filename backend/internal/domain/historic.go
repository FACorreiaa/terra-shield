package domain

import "time"

// HistoricKind is a filter group on the historic layer (L4, ADR 30).
type HistoricKind string

const (
	// HistoricMilitary: forts, fortifications, bunkers, barracks, arsenals,
	// hillforts and military bases no longer in use.
	HistoricMilitary HistoricKind = "military"
	// HistoricCastle has its own filter (owner, 2026-11-25).
	HistoricCastle HistoricKind = "castle"
	// HistoricMemorial: military museums and war cemeteries.
	HistoricMemorial HistoricKind = "memorial"
	// HistoricGeneral: landmarks that are not military heritage (L4 step 2,
	// owner 2026-11-25): palaces, cathedrals, basilicas, monasteries,
	// temples, shrines, mausoleums, manor houses, town halls, lighthouses,
	// gates, towers, windmills, Roman monuments, and every cultural World
	// Heritage Site not in another kind.
	HistoricGeneral HistoricKind = "general"
)

// HistoricKinds lists every kind, in the order clients show them.
var HistoricKinds = []HistoricKind{HistoricMilitary, HistoricCastle, HistoricMemorial, HistoricGeneral}

// ParseHistoricKind reads a kind from a query parameter.
func ParseHistoricKind(s string) (HistoricKind, bool) {
	for _, k := range HistoricKinds {
		if string(k) == s {
			return k, true
		}
	}
	return "", false
}

// HistoricSite is one item on the historic layer: a Wikidata item with
// coordinates. Static reference data, not an event.
type HistoricSite struct {
	QID         string
	Name        string
	Description string
	Kind        HistoricKind
	Class       string // Wikidata class in English ("star fort")
	Lon, Lat    float64
	ISO2        string // from Shield's geometry, else Wikidata's country
	// WorldHeritage: the item is designated a UNESCO World Heritage Site
	// (or a listed part of one) on Wikidata, whatever its kind.
	WorldHeritage bool
	// Sitelinks: how many Wikipedias and other Wikimedia wikis have a page
	// on it, a notability proxy that ranks a view holding too many sites.
	Sitelinks int
	UpdatedAt time.Time
}

// HistoricQuery filters the layer. Empty Kinds means all of them.
type HistoricQuery struct {
	BBox          *BBox
	Kinds         []HistoricKind
	WorldHeritage bool // only World Heritage Sites
}

// HistoricCell is the sites in one grid cell, for the zoomed-out view.
type HistoricCell struct {
	Lat, Lon float64 // cell center
	Count    int
	Kinds    map[HistoricKind]int
}

package domain

// CountryStats is a country with its active events at a glance.
type CountryStats struct {
	ISO2         string
	ISO3         string
	Name         string
	ActiveEvents int
	MaxSeverity  string     // "" when there are no active events
	BBox         [4]float64 // minLon, minLat, maxLon, maxLat
}

// SeverityCount is one cell of a country's type × severity matrix.
type SeverityCount struct {
	Type     EventType
	Severity string
	Count    int
}

// Coverage says which sources cover a country and whether any of them
// carries official national warnings (ADR 7).
type Coverage struct {
	OfficialWarnings bool
	Sources          []string
}

// EventChange is one revision of an event, with the event's title.
type EventChange struct {
	EventID string
	Title   string
	Revision
}

// CountrySummary is what is happening in one country.
type CountrySummary struct {
	Country       CountryStats
	Counts        []SeverityCount
	Coverage      Coverage
	Events        []EventRecord // active, most severe first
	RecentChanges []EventChange // newest first
}

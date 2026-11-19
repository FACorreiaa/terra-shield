package domain

import "time"

// SourceInfo is a feed's static metadata. The worker writes it to the
// sources table on start, so /v1/sources and the attribution screen always
// match the code.
type SourceInfo struct {
	ID           string // stable key: "usgs", "gdacs", ...
	Name         string
	URL          string // human-facing homepage of the feed
	License      string
	Attribution  string
	PollInterval time.Duration
	Enabled      bool
	// Coverage lists the ISO2 codes the source covers; empty means the
	// whole world (ADR 7).
	Coverage []string
	// OfficialWarnings marks feeds of official national warnings
	// (MeteoAlarm), as opposed to global observation feeds.
	OfficialWarnings bool
	// Scope is one line on what the source covers ("Earthquakes M2.5+
	// worldwide"). Free text for the Sources screen; unrelated to the
	// Coverage country list.
	Scope string
	// TermsURL is the source's license or terms page; empty when it could
	// not be verified.
	TermsURL string
	// OnDemand marks a source the API fetches while someone watches
	// (adsb.lol, ADR 28) instead of one the worker polls. It is never
	// stale: an unwatched layer is not a failing one.
	OnDemand bool
	// Cadence is how often the source itself publishes ("monthly
	// release"), as opposed to how often Shield polls it.
	Cadence string
	// NonHazard marks a source whose events are not hazards (UCDP's
	// conflicts, ADR 27): it is not counted as a country's coverage.
	NonHazard bool
}

// Validators are HTTP cache validators from the last good fetch, sent back
// as If-None-Match / If-Modified-Since.
type Validators struct {
	ETag         string
	LastModified string
}

// IsZero reports whether no validator is held.
func (v Validators) IsZero() bool { return v == Validators{} }

// SourceStatus is a source's metadata plus its polling health.
type SourceStatus struct {
	SourceInfo
	LastSuccessAt *time.Time
	LastErrorAt   *time.Time
	LastError     string
}

// staleAfter is how many missed intervals make a source stale; it matches
// the staleness alert (ADR 16).
const staleAfter = 3

// Stale reports whether an enabled source has gone more than three poll
// intervals without a success.
func (s SourceStatus) Stale(now time.Time) bool {
	if !s.Enabled || s.OnDemand {
		return false
	}
	return s.LastSuccessAt == nil || now.Sub(*s.LastSuccessAt) > staleAfter*s.PollInterval
}

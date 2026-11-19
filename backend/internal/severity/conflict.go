package severity

// Conflict severity from an event's best fatality estimate (UCDP's
// "best"), for armed-conflict events (ADR 27). UCDP counts only events
// with at least one direct death in its definition, though some carry a
// best estimate of 0. The bands follow the September 2026 Candidate
// release (2,014 events: median 1, 90th percentile 6, 99th 44):
//
//   - 0–4 deaths: minor (87% of events)
//   - 5–24: moderate (11%)
//   - 25–99: severe (1.5%)
//   - ≥ 100: extreme (0.5%; a massacre, a major battle)
//
// The count is a research estimate, often revised, so it is the whole
// basis of the level; there is no official alert to floor it.

// ConflictFatalities maps a best fatality estimate to a level.
func ConflictFatalities(best int) Level {
	switch {
	case best >= 100:
		return Extreme
	case best >= 25:
		return Severe
	case best >= 5:
		return Moderate
	}
	return Minor
}

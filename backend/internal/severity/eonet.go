package severity

// Wildfire severity from burned area, for incident feeds with no alert
// level of their own (EONET's IRWIN, InciWeb, BC Wildfire, ...). Bands
// follow US NWCG size classes and the common "megafire" threshold:
//
//   - < 1,000 acres (classes A–E): minor
//   - 1,000–9,999 acres (class F, small G): moderate
//   - 10,000–99,999 acres: severe
//   - ≥ 100,000 acres (megafire): extreme
//
// Burned area says nothing about the people nearby, so these bands are
// provisional until impact data (GDACS, PAGER-like) exists for the fire.

// AcresPerHectare converts hectares, the unit most non-US feeds use.
const AcresPerHectare = 2.4710538

// WildfireAcres maps burned area in acres to a level.
func WildfireAcres(acres float64) Level {
	switch {
	case acres >= 100_000:
		return Extreme
	case acres >= 10_000:
		return Severe
	case acres >= 1_000:
		return Moderate
	}
	return Minor
}

package severity

import "strings"

// GDACS alert levels estimate humanitarian impact (hazard × exposed
// population × vulnerability), not raw physical size:
//
//   - Green: low impact, local response expected → minor.
//   - Orange: medium impact, international assistance may be needed →
//     severe ("significant threat to life or property" in CAP terms).
//   - Red: high impact, international assistance likely needed → extreme.
//
// GDACS has no level between Green and Orange, so it never yields moderate.

// GDACSAlert maps a GDACS alert level (case-insensitive); ok is false for
// an unknown or empty level.
func GDACSAlert(level string) (l Level, ok bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "green":
		return Minor, true
	case "orange":
		return Severe, true
	case "red":
		return Extreme, true
	}
	return 0, false
}

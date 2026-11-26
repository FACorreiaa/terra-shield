package severity

import "strings"

// NWSAlert maps a National Weather Service alert to a level. NWS fills
// CAP's severity field itself, product by product (a Tornado Warning is
// Extreme, a Winter Storm Warning Severe, a Wind Advisory Moderate, a Frost
// Advisory Minor), so it is taken as is. "Unknown", or anything else,
// falls back on the VTEC significance:
//
//   - W warning: severe
//   - A watch: moderate
//   - Y advisory, S statement and the rest: minor
func NWSAlert(capSeverity, significance string) Level {
	if l, ok := Parse(strings.ToLower(capSeverity)); ok {
		return l
	}
	switch significance {
	case "W":
		return Severe
	case "A":
		return Moderate
	}
	return Minor
}

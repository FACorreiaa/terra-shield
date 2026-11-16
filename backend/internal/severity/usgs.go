package severity

// Earthquake severity from USGS: the highest of three signals.
//
//   - Magnitude band: < 5.0 minor, 5.0–5.9 moderate, 6.0–6.9 severe, ≥ 7.0 extreme.
//   - PAGER alert (estimated fatalities and losses), when USGS has issued
//     one: green minor, yellow moderate, orange severe, red extreme. PAGER
//     reflects impact on people, so it can raise a moderate magnitude in a
//     dense, vulnerable area.
//   - Tsunami flag: USGS sets it for large oceanic events, meaning tsunami
//     information may exist, not that a tsunami occurred. It floors the
//     result at moderate; official tsunami warnings are separate sources.

// MagnitudeLevel maps a magnitude to its band.
func MagnitudeLevel(mag float64) Level {
	switch {
	case mag >= 7.0:
		return Extreme
	case mag >= 6.0:
		return Severe
	case mag >= 5.0:
		return Moderate
	}
	return Minor
}

// PAGERLevel maps a PAGER alert colour; ok is false for none or unknown.
func PAGERLevel(alert string) (level Level, ok bool) {
	switch alert {
	case "green":
		return Minor, true
	case "yellow":
		return Moderate, true
	case "orange":
		return Severe, true
	case "red":
		return Extreme, true
	}
	return 0, false
}

// Earthquake combines the USGS signals.
func Earthquake(mag float64, pagerAlert string, tsunami bool) Level {
	levels := []Level{MagnitudeLevel(mag)}
	if l, ok := PAGERLevel(pagerAlert); ok {
		levels = append(levels, l)
	}
	if tsunami {
		levels = append(levels, Moderate)
	}
	return Max(levels...)
}

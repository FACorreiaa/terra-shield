package severity

// MeteoalarmAwareness maps MeteoAlarm's awareness level, the one color
// scale every member service shares, to a level. It is used instead of the
// CAP severity field, which members fill differently (DWD sends "Minor"
// for yellow warnings, while MeteoAlarm's own label for yellow is
// "Moderate").
//
//   - 1 green: no warning (ok is false)
//   - 2 yellow: moderate
//   - 3 orange: severe
//   - 4 red: extreme
func MeteoalarmAwareness(level int) (Level, bool) {
	switch level {
	case 2:
		return Moderate, true
	case 3:
		return Severe, true
	case 4:
		return Extreme, true
	}
	return 0, false
}

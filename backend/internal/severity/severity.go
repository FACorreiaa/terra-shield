// Package severity derives Shield's CAP-aligned severity from each source's
// own scale (ADR 2). The source's value is always stored and shown next to
// this one; the mapping tables here are documented in docs/severity.md and
// every row is covered by a table test.
package severity

// Level is CAP's severity scale, ordered so levels compare with < and >.
type Level int

const (
	Minor    Level = iota + 1 // minimal to no known threat
	Moderate                  // possible threat to life or property
	Severe                    // significant threat to life or property
	Extreme                   // extraordinary threat to life or property
)

func (l Level) String() string {
	switch l {
	case Minor:
		return "minor"
	case Moderate:
		return "moderate"
	case Severe:
		return "severe"
	case Extreme:
		return "extreme"
	}
	return "unknown"
}

// Max returns the highest level given, or Minor for none.
func Max(levels ...Level) Level {
	out := Minor
	for _, l := range levels {
		if l > out {
			out = l
		}
	}
	return out
}

// Parse reads a level's name ("severe").
func Parse(s string) (Level, bool) {
	for l := Minor; l <= Extreme; l++ {
		if l.String() == s {
			return l, true
		}
	}
	return 0, false
}

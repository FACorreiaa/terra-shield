package httpx

import (
	"strconv"
	"strings"
)

// Negotiate picks the response type for an Accept header from offered, in
// the server's order of preference (RFC 9110 §12.5.1). Each offered type
// takes the q of the most specific range matching it (type/subtype over
// type/* over */*); the highest q wins and a tie goes to the earlier
// offer. Generated clients list every response type of an operation with
// equal weight, so a tie must pick the default, not whichever comes last.
// With no Accept, or nothing acceptable, the first offer is returned:
// clients that ask for something else get the default, not a 406.
func Negotiate(accept string, offered ...string) string {
	if len(offered) == 0 {
		return ""
	}
	if strings.TrimSpace(accept) == "" {
		return offered[0]
	}
	ranges := parseAccept(accept)
	best, bestQ := offered[0], 0.0
	for _, o := range offered {
		if q := quality(ranges, o); q > bestQ {
			best, bestQ = o, q
		}
	}
	return best
}

type mediaRange struct {
	typ, sub string
	q        float64
}

func parseAccept(accept string) []mediaRange {
	var out []mediaRange
	for _, part := range strings.Split(accept, ",") {
		fields := strings.Split(part, ";")
		mt := strings.ToLower(strings.TrimSpace(fields[0]))
		typ, sub, ok := strings.Cut(mt, "/")
		if !ok || typ == "" || sub == "" {
			continue
		}
		r := mediaRange{typ: typ, sub: sub, q: 1}
		for _, p := range fields[1:] {
			k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
			if strings.EqualFold(strings.TrimSpace(k), "q") {
				if q, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && q >= 0 && q <= 1 {
					r.q = q
				}
			}
		}
		out = append(out, r)
	}
	return out
}

// quality is the q of the most specific range matching mime; 0 when none.
func quality(ranges []mediaRange, mime string) float64 {
	typ, sub, _ := strings.Cut(strings.ToLower(mime), "/")
	q, specificity := 0.0, -1
	for _, r := range ranges {
		s := -1
		switch {
		case r.typ == typ && r.sub == sub:
			s = 2
		case r.typ == typ && r.sub == "*":
			s = 1
		case r.typ == "*" && r.sub == "*":
			s = 0
		}
		if s > specificity {
			q, specificity = r.q, s
		}
	}
	return q
}

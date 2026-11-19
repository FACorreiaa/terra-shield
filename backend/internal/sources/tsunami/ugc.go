package tsunami

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ugcStartRe = regexp.MustCompile(`^([A-Z]{2}Z)(\d{3})(?:>(\d{3}))?$`)
	ugcNextRe  = regexp.MustCompile(`^(\d{3})(?:>(\d{3}))?$`)
	ugcTimeRe  = regexp.MustCompile(`^\d{6}$`)
)

// ExpandUGC turns an NWS UGC zone string into zone codes:
// "HIZ001-003-006>007-240439-" is HIZ001, HIZ003, HIZ006 and HIZ007. A
// code with a prefix starts a run that later bare numbers continue; ">"
// is an inclusive range; the six-digit group at the end is the product's
// expiry (DDHHMM) and is not a zone. County codes (SSC) are skipped:
// tsunami products use zones. A malformed group is an error.
func ExpandUGC(s string) ([]string, error) {
	var out []string
	prefix := ""
	add := func(from, to string) error {
		a, _ := strconv.Atoi(from)
		b := a
		if to != "" {
			b, _ = strconv.Atoi(to)
		}
		if b < a || b-a > 999 {
			return fmt.Errorf("ugc range %s>%s", from, to)
		}
		for n := a; n <= b; n++ {
			out = append(out, fmt.Sprintf("%s%03d", prefix, n))
		}
		return nil
	}
	for _, tok := range strings.Split(strings.TrimSpace(s), "-") {
		tok = strings.TrimSpace(tok)
		switch {
		case tok == "":
		case ugcTimeRe.MatchString(tok):
			// expiry DDHHMM
		case ugcStartRe.MatchString(tok):
			m := ugcStartRe.FindStringSubmatch(tok)
			prefix = m[1]
			if err := add(m[2], m[3]); err != nil {
				return nil, err
			}
		case len(tok) >= 3 && tok[2] == 'C':
			prefix = "" // a county run: skipped until the next zone prefix
		case ugcNextRe.MatchString(tok):
			if prefix == "" {
				continue // continues a county run
			}
			m := ugcNextRe.FindStringSubmatch(tok)
			if err := add(m[1], m[2]); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("ugc group %q", tok)
		}
	}
	return out, nil
}

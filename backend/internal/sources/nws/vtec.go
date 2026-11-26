package nws

import (
	"fmt"
	"regexp"
	"time"
)

// vtec is one P-VTEC string, NWS's machine code for a hazard event
// (NWS Directive 10-1703): "/O.CON.KPAH.WW.Y.0002.000000T0000Z-261126T1800Z/"
// is an operational message continuing Paducah's winter weather advisory
// number 2.
type vtec struct {
	class        string // O operational; T test, E and X experimental
	action       string // NEW, CON, EXT, EXA, EXB, UPG, CAN, EXP, COR, ROU
	office       string // KPAH
	phenomena    string // WW
	significance string // W warning, A watch, Y advisory, S statement, ...
	etn          string // event tracking number, per office, phenomenon and significance, reset each year
	end          time.Time
}

var vtecRe = regexp.MustCompile(`^/([OTEX])\.([A-Z]{3})\.([A-Z]{4})\.([A-Z]{2})\.([A-Z])\.(\d{4})\.(\d{6}T\d{4}Z)-(\d{6}T\d{4}Z)/$`)

// vtecTime is VTEC's yymmddThhmmZ; all zeros means "not given" (32% of
// begin times on 2026-11-26: the event is already in effect).
const vtecTime = "060102T1504Z"

func parseVTEC(s string) (vtec, error) {
	m := vtecRe.FindStringSubmatch(s)
	if m == nil {
		return vtec{}, fmt.Errorf("vtec %q", s)
	}
	v := vtec{class: m[1], action: m[2], office: m[3], phenomena: m[4], significance: m[5], etn: m[6]}
	if m[8] != "000000T0000Z" {
		end, err := time.Parse(vtecTime, m[8])
		if err != nil {
			return vtec{}, fmt.Errorf("vtec %q: end: %w", s, err)
		}
		v.end = end
	}
	return v, nil
}

// key names the event across messages: every segment of every message
// about it carries the same office, phenomenon, significance and number.
// The number restarts each year, so the stored id adds the year Shield
// first saw the event (see Source.Poll).
func (v vtec) key() string {
	return v.office + "." + v.phenomena + "." + v.significance + "." + v.etn
}

// over: the segment ends the event for its zones. A cancellation, an
// expiry, or an upgrade (the event goes on under a higher significance,
// with a key of its own).
func (v vtec) over() bool {
	return v.action == "CAN" || v.action == "EXP" || v.action == "UPG"
}

// significances in words, for source_severity.
var significances = map[string]string{
	"W": "warning", "A": "watch", "Y": "advisory", "S": "statement",
	"F": "forecast", "O": "outlook", "N": "synopsis",
}

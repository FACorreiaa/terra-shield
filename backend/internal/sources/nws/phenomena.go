package nws

import (
	"strings"

	"shield/internal/domain"
)

// phenomena maps VTEC phenomenon codes (NWS Directive 10-1703) to Shield
// types. Marine-only codes are left out: their alerts name marine zones,
// which are skipped before this table is read. TS (tsunami) is left out on
// purpose: the tsunami source owns those products (ADR 25). A code missing
// here is stored as "other" and logged, never dropped.
var phenomena = map[string]domain.EventType{
	"AF": domain.TypeVolcano,         // ashfall
	"AS": domain.TypeOther,           // air stagnation
	"BH": domain.TypeOther,           // beach hazards
	"BZ": domain.TypeSnowIce,         // blizzard
	"CF": domain.TypeFlood,           // coastal flood
	"DF": domain.TypeFlood,           // debris flow
	"DS": domain.TypeWind,            // dust storm
	"DU": domain.TypeWind,            // blowing dust
	"EC": domain.TypeCold,            // extreme cold
	"EH": domain.TypeHeat,            // excessive heat
	"EW": domain.TypeWind,            // extreme wind
	"FA": domain.TypeFlood,           // areal flood
	"FF": domain.TypeFlood,           // flash flood
	"FG": domain.TypeOther,           // dense fog
	"FL": domain.TypeFlood,           // river flood
	"FR": domain.TypeCold,            // frost
	"FW": domain.TypeWildfire,        // fire weather (Red Flag Warning): fire danger, not a fire
	"FZ": domain.TypeCold,            // freeze
	"HT": domain.TypeHeat,            // heat
	"HU": domain.TypeTropicalCyclone, // hurricane
	"HW": domain.TypeWind,            // high wind
	"HY": domain.TypeFlood,           // hydrologic
	"HZ": domain.TypeCold,            // hard freeze
	"IS": domain.TypeSnowIce,         // ice storm
	"LE": domain.TypeSnowIce,         // lake effect snow
	"LS": domain.TypeFlood,           // lakeshore flood
	"LW": domain.TypeWind,            // lake wind
	"RP": domain.TypeOther,           // rip currents
	"SM": domain.TypeOther,           // dense smoke
	"SQ": domain.TypeSnowIce,         // snow squall
	"SS": domain.TypeFlood,           // storm surge
	"SU": domain.TypeOther,           // high surf
	"SV": domain.TypeStorm,           // severe thunderstorm
	"TO": domain.TypeStorm,           // tornado
	"TR": domain.TypeTropicalCyclone, // tropical storm
	"TY": domain.TypeTropicalCyclone, // typhoon
	"WC": domain.TypeCold,            // wind chill
	"WI": domain.TypeWind,            // wind
	"WS": domain.TypeSnowIce,         // winter storm
	"WW": domain.TypeSnowIce,         // winter weather
	"XH": domain.TypeHeat,            // extreme heat
	"ZF": domain.TypeOther,           // freezing fog
	"ZR": domain.TypeSnowIce,         // freezing rain
}

// marinePrefixes are the UGC prefixes of NWS marine zones: coastal and
// offshore waters, the Great Lakes and the St. Lawrence. Shield draws land
// only, and holds no marine zone shapes.
var marinePrefixes = map[string]bool{
	"AM": true, "AN": true, "GM": true, "PZ": true, "PK": true, "PH": true, "PM": true,
	"PS": true, "LC": true, "LE": true, "LH": true, "LM": true, "LO": true, "LS": true, "SL": true,
}

func marine(ugc string) bool { return len(ugc) == 6 && ugc[2] == 'Z' && marinePrefixes[ugc[:2]] }

// territories have country codes of their own; every other UGC prefix is
// a US state or the District of Columbia. FM, MH and PW (the freely
// associated states) are independent countries whose zones NWS Guam
// warns for.
var territories = map[string]bool{
	"AS": true, "GU": true, "MP": true, "PR": true, "VI": true, "UM": true,
	"FM": true, "MH": true, "PW": true,
}

// countries are the ISO codes of the places an alert's zones are in. NWS
// zones and counties stop at the border, so country assignment follows
// them, not the drawn shape (whose edges may cross into Canada or Mexico).
func countries(ugcs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range ugcs {
		if len(u) < 2 {
			continue
		}
		iso := "US"
		if p := strings.ToUpper(u[:2]); territories[p] {
			iso = p
		}
		if !seen[iso] {
			seen[iso] = true
			out = append(out, iso)
		}
	}
	return out
}

// Coverage is where NWS issues official warnings: the states, the
// territories and the freely associated states (ADR 7).
var Coverage = []string{"US", "AS", "GU", "MP", "PR", "VI", "UM", "FM", "MH", "PW"}

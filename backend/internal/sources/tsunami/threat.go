package tsunami

import (
	"regexp"
	"strings"

	"shield/internal/severity"
)

// Band is one line of a PTWC threat forecast: the coasts expected to see
// waves of one height range.
type Band struct {
	Label     string         // "over 3 m", "1–3 m", "0.3–1 m", "hazardous waves possible"
	Level     severity.Level // owner, 2026-11-19
	Names     []string       // as PTWC writes them, title-cased: "Russia", "Northwestern Hawaiian Islands"
	Countries []string       // ISO 3166-1 alpha-2, deduplicated, in order
	Unmatched []string       // names with no code in the table
}

// A forecast bullet starts with "*" and runs to the blank line after its
// list of places. Bands below 0.3 m are not a hazard and are dropped.
var (
	bulletRe  = regexp.MustCompile(`(?m)^\s*\*\s`)
	bandRules = []struct {
		re    *regexp.Regexp
		label string
		level severity.Level
		keep  bool
	}{
		{regexp.MustCompile(`MORE THAN 3 METERS`), "over 3 m", severity.Extreme, true},
		{regexp.MustCompile(`\b1 TO 3 METERS`), "1–3 m", severity.Severe, true},
		{regexp.MustCompile(`0\.3 TO 1 METERS`), "0.3–1 m", severity.Moderate, true},
		{regexp.MustCompile(`LESS THAN 0\.3 METERS`), "under 0.3 m", severity.Minor, false},
		// The first message, before a forecast is computed, names the
		// coasts near the epicenter without a height. Not in the owner's
		// table: rated severe as "hazardous" waves, flagged for review.
		{regexp.MustCompile(`HAZARDOUS TSUNAMI WAVES FROM THIS EARTHQUAKE ARE POSSIBLE`), "hazardous waves possible", severity.Severe, true},
	}
	listRe = regexp.MustCompile(`(?s)COASTS OF\s*\n(.*?)(?:\n\s*\n|\z)`)
)

// ParseThreat reads the TSUNAMI THREAT FORECAST section of a PTWC threat
// message. It returns the bands worth storing, most severe first; none
// means no coast expects hazardous waves (or the threat has passed). ok
// is false when the text has no forecast section at all.
func ParseThreat(text string) (bands []Band, ok bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	start := strings.Index(text, "TSUNAMI THREAT FORECAST")
	if start < 0 {
		return nil, false
	}
	section := text[start:]
	if end := strings.Index(section, "RECOMMENDED ACTIONS"); end > 0 {
		section = section[:end]
	}
	locs := bulletRe.FindAllStringIndex(section, -1)
	var out []Band
	for i, loc := range locs {
		end := len(section)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		bullet := section[loc[0]:end]
		head := strings.Join(strings.Fields(bullet), " ")
		for _, b := range bandRules {
			if !b.re.MatchString(head) {
				continue
			}
			if !b.keep {
				break
			}
			m := listRe.FindStringSubmatch(bullet)
			if m == nil {
				break
			}
			band := Band{Label: b.label, Level: b.level}
			seen := map[string]bool{}
			for _, name := range places(m[1]) {
				band.Names = append(band.Names, titleCase(name))
				iso, ok := countryCodes[name]
				if !ok {
					band.Unmatched = append(band.Unmatched, name)
					continue
				}
				if !seen[iso] {
					seen[iso] = true
					band.Countries = append(band.Countries, iso)
				}
			}
			if len(band.Names) > 0 {
				out = append(out, band)
			}
			break
		}
	}
	// Most severe first, keeping PTWC's order within a level.
	sorted := make([]Band, 0, len(out))
	for _, lvl := range []severity.Level{severity.Extreme, severity.Severe, severity.Moderate} {
		for _, b := range out {
			if b.Level == lvl {
				sorted = append(sorted, b)
			}
		}
	}
	return sorted, true
}

// places splits "CHILE... HAWAII... JAPAN... NORTHWESTERN HAWAIIAN\n
// ISLANDS... AND SOLOMON ISLANDS." into names. "RUSSIA AND JAPAN" (no
// ellipses) splits on AND, but a name that itself holds AND ("WALLIS AND
// FUTUNA") is kept whole when the table knows it.
func places(list string) []string {
	list = strings.Join(strings.Fields(list), " ")
	list = strings.TrimSuffix(list, ".")
	var out []string
	for _, part := range strings.Split(list, "...") {
		part = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "AND "))
		if part == "" {
			continue
		}
		if _, ok := countryCodes[part]; ok || !strings.Contains(part, " AND ") {
			out = append(out, part)
			continue
		}
		for _, p := range strings.Split(part, " AND ") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

var smallWords = map[string]bool{"of": true, "the": true, "and": true, "near": true, "off": true, "in": true, "to": true}

// titleCase turns PTWC's capitals into a readable name: "NORTHERN
// MARIANAS" → "Northern Marianas", "DPR OF KOREA" → "DPR of Korea".
func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		switch {
		case w == "dpr" || w == "usa" || w == "us" || w == "cnmi":
			words[i] = strings.ToUpper(w)
		case i > 0 && smallWords[w]:
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// countryCodes maps the place names PTWC's Pacific and Caribbean threat
// messages use to ISO 3166-1 alpha-2. Sub-national names map to their
// country (Hawaii → US, Chuuk → FM, Kermadec Islands → NZ). A name not
// here is kept in the forecast's text but tags no country.
var countryCodes = map[string]string{
	// Pacific: countries
	"AUSTRALIA": "AU", "BRUNEI": "BN", "CANADA": "CA", "CHILE": "CL", "CHINA": "CN", "COLOMBIA": "CO",
	"COSTA RICA": "CR", "DPR OF KOREA": "KP", "ECUADOR": "EC", "EL SALVADOR": "SV", "FIJI": "FJ",
	"GUATEMALA": "GT", "HONDURAS": "HN", "INDONESIA": "ID", "JAPAN": "JP", "KIRIBATI": "KI",
	"MALAYSIA": "MY", "MARSHALL ISLANDS": "MH", "MEXICO": "MX", "MICRONESIA": "FM", "NAURU": "NR",
	"NEW ZEALAND": "NZ", "NICARAGUA": "NI", "PALAU": "PW", "PANAMA": "PA", "PAPUA NEW GUINEA": "PG",
	"PERU": "PE", "PHILIPPINES": "PH", "REPUBLIC OF KOREA": "KR", "RUSSIA": "RU", "SAMOA": "WS",
	"SINGAPORE": "SG", "SOLOMON ISLANDS": "SB", "TAIWAN": "TW", "THAILAND": "TH", "TONGA": "TO",
	"TUVALU": "TV", "VANUATU": "VU", "VIETNAM": "VN", "TIMOR-LESTE": "TL", "EAST TIMOR": "TL",
	"UNITED STATES": "US", "USA": "US", "ANTARCTICA": "AQ", "HONG KONG": "HK", "MACAO": "MO",
	// Pacific: territories and parts
	"AMERICAN SAMOA": "AS", "COOK ISLANDS": "CK", "FRENCH POLYNESIA": "PF", "GUAM": "GU",
	"NEW CALEDONIA": "NC", "NIUE": "NU", "NORTHERN MARIANAS": "MP", "NORTHERN MARIANA ISLANDS": "MP",
	"PITCAIRN ISLANDS": "PN", "PITCAIRN": "PN", "TOKELAU": "TK", "WALLIS AND FUTUNA": "WF",
	"NORFOLK ISLAND": "NF", "EASTER ISLAND": "CL", "GALAPAGOS": "EC", "GALAPAGOS ISLANDS": "EC",
	"KERMADEC ISLANDS": "NZ", "HAWAII": "US", "NORTHWESTERN HAWAIIAN ISLANDS": "US", "ALASKA": "US",
	"CALIFORNIA": "US", "OREGON": "US", "WASHINGTON": "US", "BRITISH COLUMBIA": "CA",
	"CHUUK": "FM", "KOSRAE": "FM", "POHNPEI": "FM", "YAP": "FM",
	"HOWLAND AND BAKER": "UM", "JARVIS ISLAND": "UM", "JOHNSTON ATOLL": "UM", "JOHNSTON ISLAND": "UM",
	"MIDWAY ISLAND": "UM", "PALMYRA ISLAND": "UM", "WAKE ISLAND": "UM", "KINGMAN REEF": "UM",
	// Caribbean and Atlantic
	"ANGUILLA": "AI", "ANTIGUA": "AG", "ANTIGUA AND BARBUDA": "AG", "ARUBA": "AW", "BAHAMAS": "BS",
	"BARBADOS": "BB", "BELIZE": "BZ", "BERMUDA": "BM", "BONAIRE": "BQ", "BRAZIL": "BR",
	"BRITISH VIRGIN ISLANDS": "VG", "CAYMAN ISLANDS": "KY", "CUBA": "CU", "CURACAO": "CW",
	"DOMINICA": "DM", "DOMINICAN REPUBLIC": "DO", "FRENCH GUIANA": "GF", "GRENADA": "GD",
	"GUADELOUPE": "GP", "GUYANA": "GY", "HAITI": "HT", "JAMAICA": "JM", "MARTINIQUE": "MQ",
	"MONTSERRAT": "MS", "PUERTO RICO": "PR", "SABA": "BQ", "SAINT BARTHELEMY": "BL",
	"SAINT KITTS AND NEVIS": "KN", "SAINT LUCIA": "LC", "SAINT MARTIN": "MF", "SINT MAARTEN": "SX",
	"SAINT VINCENT AND THE GRENADINES": "VC", "ST KITTS AND NEVIS": "KN", "ST LUCIA": "LC",
	"ST VINCENT AND THE GRENADINES": "VC", "SINT EUSTATIUS": "BQ", "SURINAME": "SR",
	"TRINIDAD AND TOBAGO": "TT", "TURKS AND CAICOS": "TC", "TURKS AND CAICOS ISLANDS": "TC",
	"US VIRGIN ISLANDS": "VI", "VENEZUELA": "VE",
}

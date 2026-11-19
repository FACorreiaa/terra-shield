// Package warningareas reads MeteoAlarm's geocodes file and its alias list
// (S1.5): the shapes behind the area codes in MeteoAlarm's CAP feeds, which
// carry codes only. Both files are published on MeteoAlarm's Redistribution
// Hub under CC BY 4.0 and pinned to a commit here.
package warningareas

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"shield/internal/domain"
	"shield/internal/geo/dataset"
)

// The 2026-11-13 release: 2,019 areas in 30 countries. MeteoAlarm
// publishes a new dated file several times a year (see its Redistribution
// Hub changelog); moving to it means updating both pins and reseeding,
// which `shield migrate` does when a pin changes.
var (
	Geocodes = dataset.Pin{
		Name:     "meteoalarm_areas",
		URL:      "https://gitlab.com/meteoalarm-pm-group/documents/-/raw/b3c9e19227f53f27608d3c25ac69246ef9f0580c/MeteoAlarm_Geocodes_2026_11_13.json",
		SHA256:   "28a09e1193b55b036c52226b91f225e6fbddb578d8505a1dad5e8991b34c53a6",
		MaxBytes: 96 << 20, // 31.5 MB today
	}
	Aliases = dataset.Pin{
		Name:     "meteoalarm_area_aliases",
		URL:      "https://gitlab.com/meteoalarm-pm-group/documents/-/raw/b6acb3f8de9d2a34692c9409b54c4fae016bdaec/MeteoAlarm_Geocodes-Aliases_2026_11_13.csv",
		SHA256:   "0a9f7cec4e525118fee2101f453b185359ab08e4747a60c872497ded7e631ff7",
		MaxBytes: 4 << 20, // 39 KB today
	}
)

// Processing is how areas are prepared on load; it is part of the
// recorded dataset version, so changing it reloads them on the next
// migrate. Areas are simplified to about 100 m (0.001°): the published
// shapes carry up to 18k points each, which made a warning's union and
// country assignment take seconds, and nothing at map scale needs more.
const Processing = "simplify-0.001"

// SimplifyDegrees is the tolerance Processing names.
const SimplifyDegrees = 0.001

// Version is what geo_datasets records for a pin loaded with Processing.
func Version(p dataset.Pin) string { return p.SHA256 + "+" + Processing }

var (
	codeRe    = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{1,6}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	aliasRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
)

// ParseGeocodes reads the geocodes FeatureCollection. Any malformed
// feature fails the whole file: a half-loaded area table would silently
// drop warnings.
func ParseGeocodes(body []byte) ([]domain.WarningArea, error) {
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Properties struct {
				Code    string `json:"code"`
				Country string `json:"country"`
				Name    string `json:"name"`
				Type    string `json:"type"`
			} `json:"properties"`
			Geometry json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("geocodes: %w", err)
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) == 0 {
		return nil, errors.New("geocodes: not a non-empty FeatureCollection")
	}
	seen := make(map[string]bool, len(fc.Features))
	out := make([]domain.WarningArea, 0, len(fc.Features))
	for i, f := range fc.Features {
		p := f.Properties
		if p.Type != domain.AreaTypeEMMA || !codeRe.MatchString(p.Code) || !countryRe.MatchString(p.Country) || p.Name == "" {
			return nil, fmt.Errorf("geocodes: feature %d: bad properties %+v", i, p)
		}
		if seen[p.Code] {
			return nil, fmt.Errorf("geocodes: duplicate code %s", p.Code)
		}
		seen[p.Code] = true
		var g struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(f.Geometry, &g); err != nil || (g.Type != "Polygon" && g.Type != "MultiPolygon") {
			return nil, fmt.Errorf("geocodes: %s: geometry must be a Polygon or MultiPolygon", p.Code)
		}
		out = append(out, domain.WarningArea{Code: p.Code, Country: p.Country, Name: p.Name, Geometry: f.Geometry})
	}
	return out, nil
}

// ParseAliases reads the CODE,ALIAS_CODE,ALIAS_TYPE list. known holds the
// EMMA_IDs from the geocodes file; an alias of an unknown area is an error,
// since the two files are released together.
func ParseAliases(body []byte, known []domain.WarningArea) ([]domain.AreaAlias, error) {
	codes := make(map[string]bool, len(known))
	for _, a := range known {
		codes[a.Code] = true
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = 3
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("aliases: %w", err)
	}
	if head[0] != "CODE" || head[1] != "ALIAS_CODE" || head[2] != "ALIAS_TYPE" {
		return nil, fmt.Errorf("aliases: unexpected header %v", head)
	}
	var out []domain.AreaAlias
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("aliases: %w", err)
		}
		code, alias, typ := rec[0], rec[1], rec[2]
		if !codes[code] {
			return nil, fmt.Errorf("aliases: %s %s → unknown area %s", typ, alias, code)
		}
		if !aliasRe.MatchString(alias) || !aliasRe.MatchString(typ) || typ == domain.AreaTypeEMMA {
			return nil, fmt.Errorf("aliases: bad row %v", rec)
		}
		out = append(out, domain.AreaAlias{Type: typ, Alias: alias, Code: code})
	}
	return out, nil
}

// Package countries reads the pinned Natural Earth admin-0 dataset into
// domain.Country values; store.CountryRepository loads them (ADR 4: an
// event's countries come from its geometry).
//
// Codes follow ISO 3166-1. Natural Earth's ISO_A2_EH/ISO_A3_EH fields
// already fix France, Norway, Taiwan and Kosovo; the few features with no
// ISO code are mapped to the ISO country that contains them, or skipped
// when ISO assigns none (see overrides). Features sharing a code (Australia
// and its territories) are unioned into one MultiPolygon.
package countries

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"shield/internal/domain"
	"shield/internal/geo/dataset"
)

// Dataset is the pinned Natural Earth 1:50m admin-0 countries file,
// v5.1.2, public domain (about 3 MB).
var Dataset = dataset.Pin{
	Name:     "countries",
	URL:      "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_admin_0_countries.geojson",
	SHA256:   "3e458fc036ad0a66411f2c1e6cac49c5d7bfb81cb1123bc513b22511a2b7fdeb",
	MaxBytes: 32 << 20,
}

// override maps a Natural Earth ADMIN name to ISO codes. An empty ISO2
// skips the feature.
type override struct{ iso2, iso3 string }

var overrides = map[string]override{
	"Somaliland":      {"SO", "SOM"}, // within Somalia in ISO 3166
	"Northern Cyprus": {"CY", "CYP"}, // within Cyprus in ISO 3166
	"Kosovo":          {"XK", "XKX"}, // user-assigned code in common use; NE has no A3
	"Siachen Glacier": {},            // no ISO assignment; events there get no country
}

var (
	iso2Re = regexp.MustCompile(`^[A-Z]{2}$`)
	iso3Re = regexp.MustCompile(`^[A-Z]{3}$`)
)

type geojson struct {
	Features []struct {
		Properties struct {
			Admin   string `json:"ADMIN"`
			Name    string `json:"NAME"`
			ISOA2EH string `json:"ISO_A2_EH"`
			ISOA3EH string `json:"ISO_A3_EH"`
		} `json:"properties"`
		Geometry json.RawMessage `json:"geometry"`
	} `json:"features"`
}

// Parse reads a Natural Earth admin-0 GeoJSON FeatureCollection. skipped
// lists the ADMIN names left out on purpose.
func Parse(r io.Reader) (features []domain.Country, skipped []string, err error) {
	var fc geojson
	if err := json.NewDecoder(r).Decode(&fc); err != nil {
		return nil, nil, fmt.Errorf("decode geojson: %w", err)
	}
	for _, f := range fc.Features {
		p := f.Properties
		iso2, iso3 := p.ISOA2EH, p.ISOA3EH
		if o, ok := overrides[p.Admin]; ok {
			if o.iso2 == "" {
				skipped = append(skipped, p.Admin)
				continue
			}
			iso2, iso3 = o.iso2, o.iso3
		}
		if !iso2Re.MatchString(iso2) || !iso3Re.MatchString(iso3) {
			return nil, nil, fmt.Errorf("feature %q has no usable ISO codes (%q, %q); add an override", p.Admin, iso2, iso3)
		}
		if len(f.Geometry) == 0 || string(f.Geometry) == "null" {
			return nil, nil, fmt.Errorf("feature %q has no geometry", p.Admin)
		}
		name := p.Name
		if name == "" {
			name = p.Admin
		}
		features = append(features, domain.Country{ISO2: iso2, ISO3: iso3, Name: name, Geometry: f.Geometry})
	}
	if len(features) == 0 {
		return nil, nil, errors.New("no features")
	}
	return features, skipped, nil
}

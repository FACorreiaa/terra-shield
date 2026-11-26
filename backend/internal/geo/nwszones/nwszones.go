// Package nwszones reads the National Weather Service's zone shapefiles:
// the shapes behind the UGC codes that NWS products name instead of
// drawing. Public forecast zones (HIZ001, GUZ001, …) serve tsunami and
// weather warnings, fire weather zones (WAZ658) Red Flag Warnings, and
// counties (NCC037) watches and storm-based warnings. NWS publishes each
// as a dated shapefile, public domain, pinned here.
//
// Only what Shield needs of the format is read: polygon shapes (type 5)
// and the dBASE attribute table. The rings are handed to PostGIS as a
// MultiLineString, and ST_BuildArea there sorts them into polygons and
// holes, so the reader does not need to.
package nwszones

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"shield/internal/domain"
	"shield/internal/geo/dataset"
	"shield/internal/geo/shapefile"
)

// The 16 April 2026 release (z_16ap26): 4,157 records, 4,080 zones, every
// state and territory including Guam, American Samoa, the Northern
// Marianas, Puerto Rico and the Virgin Islands. NWS replaces the file when
// zones change (a few times a year); moving to a new one means updating
// the pin, and `shield migrate` reloads it.
var Zones = dataset.Pin{
	Name:     "nws_zones",
	URL:      "https://www.weather.gov/source/gis/Shapefiles/WSOM/z_16ap26.zip",
	SHA256:   "010dc5eab64b6e326c969e252bd016f56a037b0444df8303658707bca4cc7fd8",
	MaxBytes: 96 << 20, // 26.5 MB today
}

// FireZones is the fire weather zones of the same release (fz16ap26):
// 3,683 records. Their codes look like forecast zone codes but number a
// different map, so they are a set of their own.
var FireZones = dataset.Pin{
	Name:     "nws_fire_zones",
	URL:      "https://www.weather.gov/source/gis/Shapefiles/WSOM/fz16ap26.zip",
	SHA256:   "daeb5cf94f8cd9ee850d6d142f1d98c77e6a7234f021a6b4c7aeba5763fd7486",
	MaxBytes: 96 << 20, // 23.0 MB today
}

// Counties is the counties and equivalents of the same release (c_16ap26):
// 3,352 records. A county's UGC code is its state, "C" and the last three
// digits of its FIPS code.
var Counties = dataset.Pin{
	Name:     "nws_counties",
	URL:      "https://www.weather.gov/source/gis/Shapefiles/County/c_16ap26.zip",
	SHA256:   "503f9d697a97bdbfd8a7cfe6b3b666801f43392fa220294987a68cac0bd3f03b",
	MaxBytes: 96 << 20, // 18.9 MB today
}

// Set is one zone shapefile and how its records become UGC codes.
type Set struct {
	Kind      string // nws_zones.zone_set
	Pin       dataset.Pin
	letter    string // "Z" for zones, "C" for counties
	codeField string // the record's number: ZONE, or FIPS for counties
	nameField string
}

// The three sets an alert can name.
var (
	ForecastSet = Set{Kind: "forecast", Pin: Zones, letter: "Z", codeField: "ZONE", nameField: "NAME"}
	FireSet     = Set{Kind: "fire", Pin: FireZones, letter: "Z", codeField: "ZONE", nameField: "NAME"}
	CountySet   = Set{Kind: "county", Pin: Counties, letter: "C", codeField: "FIPS", nameField: "COUNTYNAME"}
	Sets        = []Set{ForecastSet, FireSet, CountySet}
)

// Processing is how zones are prepared on load, recorded with the pin so
// a change reloads them. 0.001° is about 100 m, as for MeteoAlarm areas.
const Processing = "simplify-0.001"

// SimplifyDegrees is the tolerance Processing names.
const SimplifyDegrees = 0.001

// Version is what geo_datasets records for the pin loaded with Processing.
func Version(p dataset.Pin) string { return p.SHA256 + "+" + Processing }

var (
	stateRe = regexp.MustCompile(`^[A-Z]{2}$`)
	zoneRe  = regexp.MustCompile(`^[0-9]{3}$`)
	fipsRe  = regexp.MustCompile(`^[0-9]{5}$`)
)

// Parse reads the zipped forecast zone shapefile.
func Parse(zipped []byte) ([]domain.ForecastZone, error) { return ForecastSet.Parse(zipped) }

// code turns a record's state and number into its UGC code.
func (s Set) code(state, num string) (string, bool) {
	if !stateRe.MatchString(state) {
		return "", false
	}
	if s.letter == "C" {
		if !fipsRe.MatchString(num) {
			return "", false
		}
		return state + "C" + num[2:], true
	}
	if !zoneRe.MatchString(num) {
		return "", false
	}
	return state + "Z" + num, true
}

// Parse reads the set's zipped shapefile. A zone spread over several
// records (77 in the 2026 forecast file) becomes one zone with all their
// rings. Any malformed record fails the whole file: a half-loaded zone
// table would draw warnings smaller than they are.
func (s Set) Parse(zipped []byte) ([]domain.ForecastZone, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	shp, err := shapefile.Member(zr, ".shp")
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	dbf, err := shapefile.Member(zr, ".dbf")
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	shapes, err := shapefile.Polygons(shp)
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	for i, rings := range shapes {
		for _, ring := range rings {
			for _, pt := range ring {
				if pt[0] < -180 || pt[0] > 180 {
					return nil, fmt.Errorf("nws zones: record %d: longitude %g out of range", i+1, pt[0])
				}
			}
		}
	}
	rows, err := shapefile.DBF(dbf, "STATE", s.codeField, s.nameField)
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	if len(rows) != len(shapes) {
		return nil, fmt.Errorf("nws zones: %d attribute rows for %d shapes", len(rows), len(shapes))
	}

	type acc struct {
		state, name string
		rings       [][][2]float64
	}
	byUGC := map[string]*acc{}
	for i, row := range rows {
		if row == nil {
			continue // deleted in the attribute table
		}
		state, num, name := row["STATE"], row[s.codeField], row[s.nameField]
		ugc, ok := s.code(state, num)
		if !ok {
			return nil, fmt.Errorf("nws zones: record %d: bad code %q/%q", i+1, state, num)
		}
		if len(shapes[i]) == 0 {
			continue // a null shape draws nothing
		}
		a := byUGC[ugc]
		if a == nil {
			a = &acc{state: state, name: name}
			byUGC[ugc] = a
		}
		a.rings = append(a.rings, shapes[i]...)
	}
	if len(byUGC) == 0 {
		return nil, errors.New("nws zones: no zones")
	}
	out := make([]domain.ForecastZone, 0, len(byUGC))
	for ugc, a := range byUGC {
		geom, err := json.Marshal(map[string]any{"type": "MultiLineString", "coordinates": a.rings})
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ForecastZone{UGC: ugc, State: a.state, Name: a.name, Rings: geom})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UGC < out[j].UGC })
	return out, nil
}

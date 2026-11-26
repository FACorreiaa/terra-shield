package nhc

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// current is CurrentStorms.json: every storm NHC and CPHC are advising
// on. activeStorms is required; an empty list means no storms.
type current struct {
	ActiveStorms *[]storm `json:"activeStorms"`
}

// storm is one entry. NHC writes the latest public advisory's figures
// (intermediate advisories included) at the top level and links each
// product with its own advisory number: the forecast cone is updated
// only with full advisories.
type storm struct {
	ID               string   `json:"id"` // al052025: basin, number, year
	Name             string   `json:"name"`
	Classification   string   `json:"classification"`
	Intensity        string   `json:"intensity"` // knots
	Pressure         string   `json:"pressure"`  // mb
	LatitudeNumeric  *float64 `json:"latitudeNumeric"`
	LongitudeNumeric *float64 `json:"longitudeNumeric"`
	MovementDir      *float64 `json:"movementDir"`   // degrees, toward
	MovementSpeed    *float64 `json:"movementSpeed"` // mph
	LastUpdate       string   `json:"lastUpdate"`
	PublicAdvisory   *product `json:"publicAdvisory"`
	ForecastGraphics *product `json:"forecastGraphics"`
	TrackCone        *product `json:"trackCone"`
}

type product struct {
	AdvNum   string `json:"advNum"`
	Issuance string `json:"issuance"`
	URL      string `json:"url"`
	ZipFile  string `json:"zipFile"`
}

var stormID = regexp.MustCompile(`^(al|ep|cp)\d{6}$`)

// parseCurrent reads the list. A storm without a well-formed id is an
// error: it may be one Shield stores, and must not end for a typo.
func parseCurrent(body []byte) ([]storm, error) {
	var c current
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("nhc: %w", err)
	}
	if c.ActiveStorms == nil {
		return nil, errors.New("nhc: no activeStorms in the list")
	}
	for i := range *c.ActiveStorms {
		s := &(*c.ActiveStorms)[i]
		s.ID = strings.ToLower(strings.TrimSpace(s.ID))
		if !stormID.MatchString(s.ID) {
			return nil, fmt.Errorf("nhc: storm %d: id %q", i+1, s.ID)
		}
	}
	return *c.ActiveStorms, nil
}

// basin is the storm's basin code: AL Atlantic, EP eastern North Pacific
// (NHC), CP central North Pacific (CPHC).
func (s *storm) basin() string { return strings.ToUpper(s.ID[:2]) }

// knots is the latest intensity; false when absent.
func (s *storm) knots() (float64, bool) {
	return present(strconv.ParseFloat(strings.TrimSpace(s.Intensity), 64))
}

// pressure is the latest central pressure in mb; false when absent.
func (s *storm) pressure() (float64, bool) {
	return present(strconv.ParseFloat(strings.TrimSpace(s.Pressure), 64))
}

// position is the latest center (lon, lat); nil when absent.
func (s *storm) position() *[2]float64 {
	if s.LatitudeNumeric == nil || s.LongitudeNumeric == nil {
		return nil
	}
	lat, lon := *s.LatitudeNumeric, *s.LongitudeNumeric
	if lat < -90 || lat > 90 || lon < -540 || lon > 540 {
		return nil
	}
	return &[2]float64{wrapLon(lon), lat}
}

// classifications names NHC's codes, for titles.
var classifications = map[string]string{
	"TD":  "Tropical Depression",
	"STD": "Subtropical Depression",
	"TS":  "Tropical Storm",
	"STS": "Subtropical Storm",
	"HU":  "Hurricane",
	"MH":  "Major Hurricane",
	"TY":  "Typhoon",
	"PTC": "Post-Tropical Cyclone",
	"PC":  "Potential Tropical Cyclone",
}

// title is "Hurricane Erin"; an unknown code reads "Tropical Cyclone".
func (s *storm) title() string {
	label, ok := classifications[strings.ToUpper(s.Classification)]
	if !ok {
		label = "Tropical Cyclone"
	}
	return strings.TrimSpace(label + " " + s.Name)
}

// issued parses an ISO time NHC writes ("2023-08-22T03:00:00.000Z").
func issued(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("nhc: time %q", s)
	}
	return t.UTC(), nil
}

package tsunami

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/severity"
)

// Alert is the part of a CAP 1.2 tsunami message (CAP-TSU) Shield reads.
type Alert struct {
	Identifier string `xml:"identifier"`
	Sent       string `xml:"sent"`
	Status     string `xml:"status"`
	MsgType    string `xml:"msgType"`
	Info       []Info `xml:"info"`
}

// Info is one CAP info block.
type Info struct {
	Event       string  `xml:"event"`
	Severity    string  `xml:"severity"`
	Expires     string  `xml:"expires"`
	Headline    string  `xml:"headline"`
	Description string  `xml:"description"`
	Web         string  `xml:"web"`
	Parameters  []Param `xml:"parameter"`
	Areas       []struct {
		Desc string `xml:"areaDesc"`
	} `xml:"area"`
}

// Param is a CAP name/value pair.
type Param struct {
	Name  string `xml:"valueName"`
	Value string `xml:"value"`
}

// ParseCAP decodes one message.
func ParseCAP(body []byte) (Alert, error) {
	var a Alert
	if err := xml.Unmarshal(body, &a); err != nil {
		return Alert{}, fmt.Errorf("tsunami: cap: %w", err)
	}
	if a.Identifier == "" {
		return Alert{}, fmt.Errorf("tsunami: cap without identifier")
	}
	return a, nil
}

func (in *Info) param(name string) string {
	for _, p := range in.Parameters {
		if p.Name == name {
			return strings.TrimSpace(p.Value)
		}
	}
	return ""
}

// Outcome is what one message means for its product: store an event,
// end the one stored, or nothing.
type Outcome struct {
	Event *domain.Event // to upsert
	End   bool          // resolve Key at At
	At    time.Time
	// Zones are the UGC codes a US product names; the source draws them.
	Zones []string
	// NeedsText: a threat message whose heights are only in the bulletin
	// text (CAP carries one area); call Threat with it.
	NeedsText bool
}

// US products: the alert level, from the CAP event (owner, 2026-11-19).
var usLevels = map[string]severity.Level{
	"Tsunami Warning":  severity.Extreme,
	"Tsunami Advisory": severity.Severe,
	"Tsunami Watch":    severity.Moderate,
}

// threatExpiry bounds a threat forecast PTWC stops updating without a
// final message: its CAP has no expiry. PTWC updates hourly or so.
const threatExpiry = 12 * time.Hour

// Classify reads one message for entry. A test or exercise means nothing;
// a cancellation, a final message (no info), or a level that is not a
// hazard (Information) ends the product's event.
func Classify(e Entry, a *Alert) (Outcome, error) {
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil {
		return Outcome{}, fmt.Errorf("tsunami: %s sent %q", a.Identifier, a.Sent)
	}
	sent = sent.UTC()
	if !strings.EqualFold(a.Status, "Actual") {
		return Outcome{}, nil
	}
	if len(a.Info) == 0 || strings.EqualFold(a.MsgType, "Cancel") {
		return Outcome{End: true, At: sent}, nil
	}
	in := &a.Info[0]
	event := strings.TrimSpace(in.Event)
	if strings.Contains(event, "Cancellation") {
		return Outcome{End: true, At: sent}, nil
	}
	if ugc := in.param("NWSUGC"); ugc != "" {
		level, ok := usLevels[event]
		if !ok {
			return Outcome{End: true, At: sent}, nil // Information, or anything not a hazard
		}
		zones, err := ExpandUGC(ugc)
		if err != nil {
			return Outcome{}, fmt.Errorf("tsunami: %s: %w", a.Identifier, err)
		}
		ev, err := base(e, a, in, sent)
		if err != nil {
			return Outcome{}, err
		}
		ev.Kind = domain.KindWarning
		ev.Severity = level.String()
		place := usPlace(in)
		ev.Title = event + " · " + place
		ev.Summary = fmt.Sprintf("%s for %s after %s.", event, place, quake(in))
		if exp, err := time.Parse(time.RFC3339, strings.TrimSpace(in.Expires)); err == nil {
			ev.Attrs["expiresAt"] = exp.UTC()
		}
		ev.Attrs["zones"] = zones
		return Outcome{Event: &ev, Zones: zones}, nil
	}
	if strings.HasPrefix(event, "Tsunami Information") {
		return Outcome{End: true, At: sent}, nil
	}
	return Outcome{NeedsText: true, At: sent}, nil
}

// Threat builds the forecast event of a PTWC threat message from its
// bulletin text, or ends it when no coast expects hazardous waves.
func Threat(e Entry, a *Alert, text string) (Outcome, error) {
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil || len(a.Info) == 0 {
		return Outcome{}, fmt.Errorf("tsunami: %s: not a threat message", a.Identifier)
	}
	sent = sent.UTC()
	in := &a.Info[0]
	bands := ParseThreat(text)
	if len(bands) == 0 {
		return Outcome{End: true, At: sent}, nil
	}
	ev, err := base(e, a, in, sent)
	if err != nil {
		return Outcome{}, err
	}
	ev.Kind = domain.KindForecast
	ev.Severity = bands[0].Level.String()
	var countries, unmatched []string
	seen := map[string]bool{}
	wire := make([]map[string]any, 0, len(bands))
	var parts []string
	for _, b := range bands {
		for _, c := range b.Countries {
			if !seen[c] {
				seen[c] = true
				countries = append(countries, c)
			}
		}
		unmatched = append(unmatched, b.Unmatched...)
		wire = append(wire, map[string]any{"band": b.Label, "severity": b.Level.String(), "places": b.Names, "countries": b.Countries})
		parts = append(parts, b.Label+": "+strings.Join(b.Names, ", "))
	}
	ev.CountryRule = domain.CountryRule{Listed: countries}
	top := bands[0].Names
	ev.Title = "Tsunami threat forecast · " + top[0]
	if len(top) > 1 {
		ev.Title += fmt.Sprintf(" +%d", len(top)-1)
	}
	ev.Summary = fmt.Sprintf("Guidance for national authorities, not a warning: PTWC forecasts waves after %s. %s. "+
		"National authorities decide each country's alert.", quake(in), strings.Join(parts, "; "))
	ev.Attrs["forecast"] = wire
	ev.Attrs["guidance"] = true
	ev.Attrs["expiresAt"] = sent.Add(threatExpiry)
	if len(unmatched) > 0 {
		ev.Attrs["unmatchedPlaces"] = unmatched
	}
	return Outcome{Event: &ev}, nil
}

// base fills what every stored tsunami event shares; geometry is the
// epicenter until the caller draws zones.
func base(e Entry, a *Alert, in *Info, sent time.Time) (domain.Event, error) {
	lat, lon, err := latLon(in.param("EventLatLon"))
	if err != nil {
		return domain.Event{}, fmt.Errorf("tsunami: %s: %w", a.Identifier, err)
	}
	origin := sent
	if t, err := time.Parse(time.RFC3339, in.param("EventOriginTime")); err == nil {
		origin = t.UTC()
	}
	attrs := map[string]any{
		"center":    centers[e.Center],
		"product":   e.Product,
		"bulletin":  e.Bulletin,
		"incident":  e.Incident,
		"epicenter": []float64{lon, lat},
		"geometry":  "epicenter",
	}
	if m, err := strconv.ParseFloat(in.param("EventPreliminaryMagnitude"), 64); err == nil {
		attrs["magnitude"] = m
		attrs["magnitudeType"] = in.param("EventPreliminaryMagnitudeType")
	}
	return domain.Event{
		SourceEventID:  e.Key(),
		Type:           domain.TypeTsunami,
		Status:         domain.StatusActive,
		SourceSeverity: strings.TrimSpace(in.Event),
		Geometry:       domain.PointGeometry(lon, lat),
		StartedAt:      &origin,
		ObservedAt:     sent,
		UpdatedAt:      sent,
		URL:            bulletinURL(e),
		Attrs:          attrs,
	}, nil
}

var centers = map[string]string{"PHEB": "PTWC", "PAAQ": "NTWC"}

// latLon reads "52.209,159.979 0.000" (lat,lon depth).
func latLon(s string) (float64, float64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, 0, fmt.Errorf("no EventLatLon")
	}
	ll := strings.Split(f[0], ",")
	if len(ll) != 2 {
		return 0, 0, fmt.Errorf("EventLatLon %q", s)
	}
	lat, err1 := strconv.ParseFloat(ll[0], 64)
	lon, err2 := strconv.ParseFloat(ll[1], 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("EventLatLon %q", s)
	}
	return lat, lon, nil
}

// quake describes the earthquake: "a preliminary magnitude 8.7 earthquake
// off the east coast of Kamchatka, Russia".
func quake(in *Info) string {
	loc := location(in.param("EventLocationName"))
	if loc != "" {
		loc = " " + lowerFirst(loc)
	}
	mag := in.param("EventPreliminaryMagnitude")
	if mag == "" {
		return "an earthquake" + loc
	}
	return "a preliminary magnitude " + mag + " earthquake" + loc
}

// directionWords stay lower case inside a location.
var directionWords = map[string]bool{
	"north": true, "south": true, "east": true, "west": true, "northeast": true, "northwest": true,
	"southeast": true, "southwest": true, "northern": true, "southern": true, "eastern": true,
	"western": true, "central": true, "coast": true, "region": true, "about": true, "miles": true,
	"km": true, "of": true, "the": true, "off": true, "near": true,
}

// location renders PTWC's "OFF THE EAST COAST OF KAMCHATKA  RUSSIA" as
// "Off the east coast of Kamchatka, Russia": a double space separates a
// place from its country.
func location(s string) string {
	parts := strings.Split(strings.TrimSpace(s), "  ")
	var out []string
	for _, p := range parts {
		words := strings.Fields(titleCase(p))
		for i, w := range words {
			if i > 0 && directionWords[strings.ToLower(w)] {
				words[i] = strings.ToLower(w)
			}
		}
		if len(words) > 0 {
			out = append(out, strings.Join(words, " "))
		}
	}
	return strings.Join(out, ", ")
}

// lowerFirst keeps a location that starts with a preposition reading as
// one ("off the east coast of …"), and prefixes "near" to a bare place.
func lowerFirst(s string) string {
	first := strings.ToLower(strings.Fields(s)[0])
	switch first {
	case "off", "near", "south", "north", "east", "west", "about", "in", "southern", "northern", "eastern", "western", "central":
		return strings.ToLower(s[:1]) + s[1:]
	}
	if strings.HasPrefix(first, "about") || strings.Contains(strings.ToLower(s), " of ") {
		return strings.ToLower(s[:1]) + s[1:]
	}
	return "near " + s
}

// usPlace is the coast a US product covers: "ALL COUNTIES IN THE STATE OF
// HAWAII" → "Hawaii"; "Guam and CNMI" stays.
func usPlace(in *Info) string {
	desc := ""
	if len(in.Areas) > 0 {
		desc = strings.TrimSpace(in.Areas[0].Desc)
	}
	upper := strings.ToUpper(desc)
	for _, p := range []string{"ALL COUNTIES IN THE STATE OF ", "ALL COUNTIES IN "} {
		if strings.HasPrefix(upper, p) {
			desc = desc[len(p):]
			break
		}
	}
	if desc == strings.ToUpper(desc) {
		desc = titleCase(desc)
	}
	if desc == "" {
		return "US coasts"
	}
	return desc
}

func bulletinURL(e Entry) string {
	return "https://www.tsunami.gov" + e.Path + "/" + e.Product + "/" + e.Product + ".txt"
}

func capURL(base string, e Entry) string {
	return base + e.Path + "/" + e.Product + "/" + e.Center + "CAP.xml"
}

func textURL(base string, e Entry) string {
	return base + e.Path + "/" + e.Product + "/" + e.Product + ".txt"
}

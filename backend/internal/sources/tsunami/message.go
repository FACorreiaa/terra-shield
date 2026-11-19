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

// Outcome is what one message means for its product. Every message is a
// snapshot of the product's state: what Events holds is in force, and
// every other event of the product has ended, at At.
type Outcome struct {
	Events []Draft
	At     time.Time
	// Ignore: a test or exercise; nothing changes.
	Ignore bool
	// NeedsText: a PTWC threat message, whose wave heights are only in the
	// bulletin text; call Threat with it.
	NeedsText bool
}

// Draft is an event to store, with the NWS zones (UGC) to draw it from.
type Draft struct {
	Event domain.Event
	Zones []string
}

// US alert levels, from the CAP event (owner, 2026-11-19). CAP's own
// severity rates Advisory and Watch alike (Moderate), so it is not used.
var usLevels = map[string]struct {
	level severity.Level
	slug  string
}{
	"Tsunami Warning":  {severity.Extreme, "warning"},
	"Tsunami Advisory": {severity.Severe, "advisory"},
	"Tsunami Watch":    {severity.Moderate, "watch"},
}

// levelSlugs, most severe first: a product's US events are its key plus
// one of these.
var levelSlugs = []string{"warning", "advisory", "watch"}

// threatProducts are PTWC's threat messages: Pacific, Caribbean.
var threatProducts = map[string]bool{"WEPA40": true, "WECA41": true, "WECA43": true}

// ProductKeys are every event id one product can hold: its threat
// forecast and one US event per level.
func ProductKeys(e Entry) []string {
	keys := []string{e.Key()}
	for _, l := range levelSlugs {
		keys = append(keys, e.Key()+"-"+l)
	}
	return keys
}

// threatExpiry bounds a threat forecast PTWC stops updating without a
// final message: its CAP has no expiry. PTWC updates hourly or so.
const threatExpiry = 12 * time.Hour

// Classify reads one message for entry. US products carry one <info> per
// coastal segment, each with its own level and zones (NTWC's West Coast
// and Alaska product mixed Warning, Advisory and Cancellation segments in
// one message); segments of one level make one event. Canceled segments
// and information statements are not hazards. An event string not known
// here is an error, never read as an end: a warning is not dropped
// because its wording changed.
func Classify(e Entry, a *Alert) (Outcome, error) {
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil {
		return Outcome{}, fmt.Errorf("tsunami: %s sent %q", a.Identifier, a.Sent)
	}
	sent = sent.UTC()
	if !strings.EqualFold(a.Status, "Actual") {
		return Outcome{Ignore: true, At: sent}, nil
	}
	if len(a.Info) == 0 || strings.EqualFold(a.MsgType, "Cancel") {
		return Outcome{At: sent}, nil // a final message: everything ends
	}
	type level struct {
		severity severity.Level
		event    string
		zones    []string
		areas    []string
		expires  time.Time
		info     *Info
	}
	levels := map[string]*level{}
	us, other := false, 0
	for i := range a.Info {
		in := &a.Info[i]
		event := strings.TrimSpace(in.Event)
		ugc := in.param("NWSUGC")
		us = us || ugc != ""
		lv, known := usLevels[event]
		switch {
		case strings.Contains(event, "Cancellation"), strings.HasPrefix(event, "Tsunami Information"):
			continue
		case known && ugc != "":
			zones, err := ExpandUGC(ugc)
			if err != nil {
				return Outcome{}, fmt.Errorf("tsunami: %s: %w", a.Identifier, err)
			}
			l := levels[lv.slug]
			if l == nil {
				l = &level{severity: lv.level, event: event, info: in}
				levels[lv.slug] = l
			}
			l.zones = append(l.zones, zones...)
			l.areas = append(l.areas, usPlace(in))
			if exp, err := time.Parse(time.RFC3339, strings.TrimSpace(in.Expires)); err == nil && exp.After(l.expires) {
				l.expires = exp.UTC()
			}
		case ugc != "" || known:
			return Outcome{}, fmt.Errorf("tsunami: %s: unknown US alert %q", a.Identifier, event)
		default:
			other++
		}
	}
	switch {
	case us && other > 0:
		return Outcome{}, fmt.Errorf("tsunami: %s mixes US and non-US segments", a.Identifier)
	case other > 0 && threatProducts[e.Product]:
		return Outcome{NeedsText: true, At: sent}, nil
	case other > 0:
		return Outcome{}, fmt.Errorf("tsunami: %s: unknown message %q in %s", a.Identifier, a.Info[0].Event, e.Product)
	}
	out := Outcome{At: sent}
	for _, slug := range levelSlugs {
		l := levels[slug]
		if l == nil {
			continue
		}
		ev, err := base(e, a, l.info, sent)
		if err != nil {
			return Outcome{}, err
		}
		ev.SourceEventID = e.Key() + "-" + slug
		ev.Kind = domain.KindWarning
		ev.Severity = l.severity.String()
		ev.SourceSeverity = l.event
		zones := dedupe(l.zones)
		ev.Title = l.event + " · " + regions(zones)
		ev.Summary = fmt.Sprintf("%s for %s after %s.", l.event, strings.Join(l.areas, "; "), quake(l.info))
		if !l.expires.IsZero() {
			ev.Attrs["expiresAt"] = l.expires
		}
		ev.Attrs["zones"] = zones
		ev.Attrs["areas"] = l.areas
		// Countries from the zones the message names, drawn or not: BC
		// zones tag Canada though NWS holds no shape for them, and a
		// drawn sliver never tags a neighbor.
		ev.CountryRule = domain.CountryRule{Listed: zoneCountries(zones)}
		out.Events = append(out.Events, Draft{Event: ev, Zones: zones})
	}
	return out, nil
}

// Threat builds the forecast event of a PTWC threat message from its
// bulletin text. A forecast naming no coast at 0.3 m or more ends it; a
// text without a forecast section is an error (the format changed).
func Threat(e Entry, a *Alert, text string) (Outcome, error) {
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil || len(a.Info) == 0 {
		return Outcome{}, fmt.Errorf("tsunami: %s: not a threat message", a.Identifier)
	}
	sent = sent.UTC()
	in := &a.Info[0]
	bands, ok := ParseThreat(text)
	if !ok {
		return Outcome{}, fmt.Errorf("tsunami: %s: bulletin has no threat forecast", a.Identifier)
	}
	if len(bands) == 0 {
		return Outcome{At: sent}, nil
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
	return Outcome{Events: []Draft{{Event: ev}}, At: sent}, nil
}

func dedupe(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := s[:0:0]
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// zoneRegions names the state or territory of a UGC prefix. Marine zones
// (PZ, AM, ...) name no region.
var zoneRegions = map[string]string{
	"AK": "Alaska", "AS": "American Samoa", "BC": "British Columbia", "CA": "California", "GU": "Guam",
	"HI": "Hawaii", "MP": "Northern Mariana Islands", "OR": "Oregon", "PR": "Puerto Rico",
	"VI": "US Virgin Islands", "WA": "Washington", "UM": "US Minor Outlying Islands",
	"FL": "Florida", "GA": "Georgia", "SC": "South Carolina", "NC": "North Carolina", "VA": "Virginia",
	"MD": "Maryland", "DE": "Delaware", "NJ": "New Jersey", "NY": "New York", "CT": "Connecticut",
	"RI": "Rhode Island", "MA": "Massachusetts", "NH": "New Hampshire", "ME": "Maine", "AL": "Alabama",
	"MS": "Mississippi", "LA": "Louisiana", "TX": "Texas",
}

// zoneCountries maps UGC prefixes to ISO 3166-1 alpha-2: US states to US,
// territories to their own code, British Columbia to CA. Marine zones and
// unknown prefixes tag nothing.
func zoneCountries(zones []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, z := range zones {
		p := z[:2]
		iso := ""
		switch {
		case p == "BC":
			iso = "CA"
		case p == "GU" || p == "MP" || p == "AS" || p == "PR" || p == "VI" || p == "UM":
			iso = p
		case zoneRegions[p] != "":
			iso = "US"
		}
		if iso != "" && !seen[iso] {
			seen[iso] = true
			out = append(out, iso)
		}
	}
	return out
}

// regions lists the states and territories a product's zones lie in, in
// the order they first appear: "California, Oregon, Washington, Alaska".
func regions(zones []string) string {
	var names []string
	seen := map[string]bool{}
	for _, z := range zones {
		if n, ok := zoneRegions[z[:2]]; ok && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return "US coasts"
	}
	return strings.Join(names, ", ")
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

// usPlace is the coast one segment of a US product covers: "ALL COUNTIES
// IN THE STATE OF HAWAII" → "Hawaii"; NTWC's "Coastal areas between and
// including Chignik Bay, Alaska to Samalga Pass, Alaska" → "Chignik Bay,
// Alaska to Samalga Pass, Alaska"; "Guam and CNMI" stays.
func usPlace(in *Info) string {
	desc := ""
	if len(in.Areas) > 0 {
		desc = strings.TrimSpace(in.Areas[0].Desc)
	}
	upper := strings.ToUpper(desc)
	for _, p := range []string{"ALL COUNTIES IN THE STATE OF ", "ALL COUNTIES IN ", "COASTAL AREAS BETWEEN AND INCLUDING "} {
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

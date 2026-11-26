package nws

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/severity"
)

// Skip reasons, for metrics.SourceSkipped.
const (
	SkipMarine  = "marine"          // marine zones only: Shield draws land
	SkipTsunami = "tsunami"         // the tsunami source owns these (ADR 25)
	SkipNotMet  = "not_met"         // CAP category other than Met (civil emergency messages)
	SkipNoVTEC  = "no_vtec"         // no event code to follow it by (statements, air quality alerts)
	SkipTest    = "not_operational" // a test or experimental VTEC
)

// errShape is a body that is not the alerts GeoJSON.
var errShape = errors.New("nws: unexpected response shape")

type collection struct {
	Features *[]struct {
		Geometry   json.RawMessage `json:"geometry"`
		Properties properties      `json:"properties"`
	} `json:"features"`
}

type properties struct {
	ID            string   `json:"id"`
	AreaDesc      string   `json:"areaDesc"`
	AffectedZones []string `json:"affectedZones"`
	Geocode       struct {
		UGC []string `json:"UGC"`
	} `json:"geocode"`
	Sent        string `json:"sent"`
	Effective   string `json:"effective"`
	Onset       string `json:"onset"`
	Ends        string `json:"ends"`
	Expires     string `json:"expires"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Certainty   string `json:"certainty"`
	Urgency     string `json:"urgency"`
	Event       string `json:"event"`
	SenderName  string `json:"senderName"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	Parameters  struct {
		VTEC        []string `json:"VTEC"`
		NWSheadline []string `json:"NWSheadline"`
	} `json:"parameters"`
}

// segment is one alert's part in one event: its zones, its text and its
// VTEC action.
type segment struct {
	v        vtec
	id       string
	p        *properties
	sent     time.Time
	start    time.Time
	ends     time.Time
	zones    []domain.ZoneRef
	polygon  json.RawMessage
	ugcs     []string
	areaDesc []string
}

// Warning is one NWS event, gathered from every listed segment still in
// force. Event has no SourceEventID or Geometry yet: Poll gives it the
// stored id (or a new one) and draws Zones and Polygons.
type Warning struct {
	Key      string // office.phenomenon.significance.number
	Event    domain.Event
	Zones    []domain.ZoneRef
	Polygons []json.RawMessage
}

// Parsed is one read of the active alerts.
type Parsed struct {
	Warnings []Warning
	// Live holds the key of every event the list still holds in force,
	// read before any filter: a listed event is never ended by a skip.
	Live map[string]bool
	// Unreadable counts alerts whose VTEC could not be read. Their events
	// cannot be told apart, so Poll ends nothing while any is listed.
	Unreadable int
	Skipped    map[string]int
	// Unknown lists phenomenon codes missing from the table, stored as
	// "other".
	Unknown []string
	Listed  int
}

// Parse reads the active alerts. An empty list is an error: NWS always
// has some in force (699 on 2026-11-26, 309 over land), and treating an
// outage that answers with none as "everything ended" would wipe the map.
func Parse(body []byte) (Parsed, error) {
	var c collection
	if err := json.Unmarshal(body, &c); err != nil {
		return Parsed{}, fmt.Errorf("%w: %w", errShape, err)
	}
	if c.Features == nil {
		return Parsed{}, errShape
	}
	if len(*c.Features) == 0 {
		return Parsed{}, fmt.Errorf("%w: no alerts listed", errShape)
	}
	out := Parsed{Live: map[string]bool{}, Skipped: map[string]int{}, Listed: len(*c.Features)}
	byKey := map[string][]segment{}
	for i := range *c.Features {
		f := &(*c.Features)[i]
		p := &f.Properties
		var vs []vtec
		for _, s := range p.Parameters.VTEC {
			v, err := parseVTEC(s)
			if err != nil {
				out.Unreadable++
				continue
			}
			vs = append(vs, v)
			if v.class == "O" && !v.over() {
				out.Live[v.key()] = true
			}
		}
		switch {
		case len(p.Parameters.VTEC) == 0:
			out.Skipped[SkipNoVTEC]++
			continue
		case p.Category != "Met":
			out.Skipped[SkipNotMet]++
			continue
		case len(p.Geocode.UGC) > 0 && allMarine(p.Geocode.UGC):
			out.Skipped[SkipMarine]++
			continue
		}
		sent, err := time.Parse(time.RFC3339, p.Sent)
		if err != nil {
			out.Unreadable++
			continue
		}
		for _, v := range vs {
			switch {
			case v.class != "O":
				out.Skipped[SkipTest]++
			case v.phenomena == "TS":
				out.Skipped[SkipTsunami]++
			case !v.over():
				byKey[v.key()] = append(byKey[v.key()], newSegment(v, p, f.Geometry, sent))
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	unknown := map[string]bool{}
	for _, k := range keys {
		w := build(k, byKey[k])
		if _, ok := phenomena[byKey[k][0].v.phenomena]; !ok && !unknown[byKey[k][0].v.phenomena] {
			unknown[byKey[k][0].v.phenomena] = true
			out.Unknown = append(out.Unknown, byKey[k][0].v.phenomena)
		}
		out.Warnings = append(out.Warnings, w)
	}
	return out, nil
}

func allMarine(ugcs []string) bool {
	for _, u := range ugcs {
		if !marine(u) {
			return false
		}
	}
	return true
}

func newSegment(v vtec, p *properties, geometry json.RawMessage, sent time.Time) segment {
	s := segment{v: v, id: p.ID, p: p, sent: sent.UTC(), start: sent.UTC()}
	for _, t := range []string{p.Onset, p.Effective} {
		if at, err := time.Parse(time.RFC3339, t); err == nil {
			s.start = at.UTC()
			break
		}
	}
	if at, err := time.Parse(time.RFC3339, p.Ends); err == nil {
		s.ends = at.UTC()
	} else if !v.end.IsZero() {
		s.ends = v.end
	}
	if g := strings.TrimSpace(string(geometry)); g != "" && g != "null" {
		s.polygon = geometry
	}
	sets := zoneSets(p.AffectedZones)
	for _, u := range p.Geocode.UGC {
		if marine(u) {
			continue
		}
		s.ugcs = append(s.ugcs, u)
		set := "forecast"
		switch {
		case len(u) == 6 && u[2] == 'C':
			set = "county"
		case sets[u] != "":
			set = sets[u]
		}
		s.zones = append(s.zones, domain.ZoneRef{Set: set, UGC: u})
	}
	for _, a := range strings.Split(p.AreaDesc, ";") {
		if a = strings.TrimSpace(a); a != "" {
			s.areaDesc = append(s.areaDesc, a)
		}
	}
	return s
}

// zoneSets reads which set each zone code belongs to from the zone URLs
// (".../zones/fire/WAZ658"): a fire zone shares its code's form with a
// forecast zone, but not its shape.
func zoneSets(urls []string) map[string]string {
	out := map[string]string{}
	for _, u := range urls {
		parts := strings.Split(u, "/")
		if len(parts) < 3 {
			continue
		}
		code, set := parts[len(parts)-1], parts[len(parts)-2]
		if set == "fire" || set == "county" || set == "forecast" {
			out[code] = set
		}
	}
	return out
}

// build gathers an event's segments. The latest message speaks for the
// event (title, text); the area is every segment's; the level the highest.
func build(key string, segs []segment) Warning {
	sort.Slice(segs, func(i, j int) bool {
		if !segs[i].sent.Equal(segs[j].sent) {
			return segs[i].sent.Before(segs[j].sent)
		}
		return segs[i].id < segs[j].id
	})
	last := segs[len(segs)-1]
	w := Warning{Key: key}
	var (
		level     severity.Level
		capLevel  string
		started   = last.start
		ends      time.Time
		areas     []string
		ugcs      []string
		seenZone  = map[domain.ZoneRef]bool{}
		seenArea  = map[string]bool{}
		seenPoly  = map[string]bool{}
		certainty = last.p.Certainty
	)
	for i := range segs {
		s := &segs[i]
		if l := severity.NWSAlert(s.p.Severity, s.v.significance); l > level {
			level, capLevel = l, s.p.Severity
		}
		if s.start.Before(started) {
			started = s.start
		}
		if s.ends.After(ends) {
			ends = s.ends
		}
		for _, a := range s.areaDesc {
			if !seenArea[a] {
				seenArea[a] = true
				areas = append(areas, a)
			}
		}
		ugcs = append(ugcs, s.ugcs...)
		if s.polygon != nil {
			// A storm-based warning's polygon is its area; the counties it
			// lists are only those the polygon touches.
			if k := string(s.polygon); !seenPoly[k] {
				seenPoly[k] = true
				w.Polygons = append(w.Polygons, s.polygon)
			}
			continue
		}
		for _, z := range s.zones {
			if !seenZone[z] {
				seenZone[z] = true
				w.Zones = append(w.Zones, z)
			}
		}
	}
	slices.SortFunc(w.Zones, func(a, b domain.ZoneRef) int { return strings.Compare(a.Set+a.UGC, b.Set+b.UGC) })
	slices.Sort(areas)
	typ, ok := phenomena[last.v.phenomena]
	if !ok {
		typ = domain.TypeOther
	}
	sig := significances[last.v.significance]
	attrs := map[string]any{
		"office":       last.v.office,
		"phenomena":    last.v.phenomena,
		"significance": last.v.significance,
		"etn":          last.v.etn,
		"event":        last.p.Event,
		"capSeverity":  capLevel,
		"certainty":    certainty,
		"urgency":      last.p.Urgency,
		"sender":       last.p.SenderName,
		"areas":        areas,
	}
	if !ends.IsZero() {
		attrs["endsAt"] = ends
	}
	w.Event = domain.Event{
		Type:           typ,
		Kind:           domain.KindWarning,
		Status:         domain.StatusActive,
		Severity:       level.String(),
		SourceSeverity: strings.TrimSpace(capLevel + " (" + sig + ")"),
		Title:          title(last.p.Event, areas),
		Summary:        summary(last.p),
		StartedAt:      &started,
		ObservedAt:     last.sent,
		UpdatedAt:      last.sent,
		URL:            officeURL(last.v.office),
		Attrs:          attrs,
		CountryRule:    domain.CountryRule{Listed: countries(ugcs)},
	}
	return w
}

// title names the event and where: "Wind Advisory: Perry, Jackson and 2
// more".
func title(event string, areas []string) string {
	const shown = 3
	switch {
	case len(areas) == 0:
		return event
	case len(areas) <= shown:
		return event + ": " + strings.Join(areas, ", ")
	default:
		return fmt.Sprintf("%s: %s and %d more", event, strings.Join(areas[:shown], ", "), len(areas)-shown)
	}
}

// summary is the latest message's own words: its headline, what it says
// and what to do.
func summary(p *properties) string {
	var parts []string
	if len(p.Parameters.NWSheadline) > 0 {
		parts = append(parts, strings.TrimSpace(p.Parameters.NWSheadline[0]))
	} else if h := strings.TrimSpace(p.Headline); h != "" {
		parts = append(parts, h)
	}
	for _, s := range []string{p.Description, p.Instruction} {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// officeURL is the issuing office's page (ADR 12): a forecast office's
// three-letter page, or a national center's site.
func officeURL(office string) string {
	switch office {
	case "KWNS":
		return "https://www.spc.noaa.gov/"
	case "KNHC":
		return "https://www.nhc.noaa.gov/"
	case "PHFO":
		return "https://www.weather.gov/hfo/"
	case "TJSJ":
		return "https://www.weather.gov/sju/"
	case "NSTU":
		return "https://www.weather.gov/ppg/"
	case "PGUM":
		return "https://www.weather.gov/gum/"
	}
	if len(office) == 4 {
		return "https://www.weather.gov/" + strings.ToLower(office[1:]) + "/"
	}
	return "https://www.weather.gov/"
}

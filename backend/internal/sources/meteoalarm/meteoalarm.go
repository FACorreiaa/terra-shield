// Package meteoalarm ingests official weather warnings for Europe from
// MeteoAlarm (EUMETNET): CAP 1.2 alerts from each member's national
// weather service, one public JSON feed per country.
//
// A warning's areas arrive as CAP areas, each with an inline polygon drawn
// by the national service, or with codes only: EMMA_IDs, or the NUTS2,
// NUTS3 or FIPS codes some members send instead. Polygons are used as
// sent; codes are drawn from MeteoAlarm's geocodes file through an
// AreaResolver (internal/geo/warningareas, store.WarningAreaRepository).
// A warning with any area that cannot be drawn is skipped, never drawn
// smaller than it is.
//
// Each feed is a snapshot of the country's recent warnings, expired ones
// included. A warning is keyed by its CAP identifier; an Update or Cancel
// names the identifiers it replaces in `references`, and those rows are
// deleted. Expired warnings are stored as resolved.
//
// License: CC BY 4.0. National members may add terms (checked before the
// external TestFlight, docs/sources-licensing.md).
package meteoalarm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/metrics"
	"shield/internal/severity"
)

const (
	// ID is the source key in the sources table.
	ID = "meteoalarm"
	// FeedBase is followed by a country slug (Country.Slug).
	FeedBase = "https://feeds.meteoalarm.org/api/v1/warnings/feeds-"
	// PollInterval: the feeds send no validators, and one round of every
	// country is about 30 MB (Germany and Switzerland alone are 5–8 MB, eight
	// languages each), so 10 minutes rather than the 5 the members update at.
	PollInterval = 10 * time.Minute
)

// PollTimeout implements ingest.SlowSource. A round reads 37 feeds (about
// 30 MB, one request a second to the same host) and stores each country;
// a first poll after a restart also draws every warning. Measured at
// about 3 minutes on a heavily loaded machine; 8 leaves room inside the
// 10-minute interval.
func (*Source) PollTimeout() time.Duration { return 8 * time.Minute }

var _ ingest.SlowSource = (*Source)(nil)

// NeighborShare is how much of a warning's area another country must hold
// to be assigned it besides the issuer (owner's call, 2026-11-19). Live,
// 603 of 702 neighbor matches were under 5%: border mismatch with Natural
// Earth. Those at 5% or more include real cover, such as MeteoSwiss
// warnings for Liechtenstein.
const NeighborShare = 0.05

// Country is one member feed.
type Country struct {
	ISO2 string
	Slug string
}

// Countries are the member feeds that answered on 2026-11-16. North
// Macedonia is a member but its feed was not found under any obvious slug.
var Countries = []Country{
	{"AT", "austria"},
	{"BE", "belgium"},
	{"BA", "bosnia-herzegovina"},
	{"BG", "bulgaria"},
	{"HR", "croatia"},
	{"CY", "cyprus"},
	{"CZ", "czechia"},
	{"DK", "denmark"},
	{"EE", "estonia"},
	{"FI", "finland"},
	{"FR", "france"},
	{"DE", "germany"},
	{"GR", "greece"},
	{"HU", "hungary"},
	{"IS", "iceland"},
	{"IE", "ireland"},
	{"IL", "israel"},
	{"IT", "italy"},
	{"LV", "latvia"},
	{"LT", "lithuania"},
	{"LU", "luxembourg"},
	{"MT", "malta"},
	{"MD", "moldova"},
	{"ME", "montenegro"},
	{"NL", "netherlands"},
	{"NO", "norway"},
	{"PL", "poland"},
	{"PT", "portugal"},
	{"RO", "romania"},
	{"RS", "serbia"},
	{"SK", "slovakia"},
	{"SI", "slovenia"},
	{"ES", "spain"},
	{"SE", "sweden"},
	{"CH", "switzerland"},
	{"GB", "united-kingdom"},
	{"UA", "ukraine"},
}

// Area is one CAP area of a warning: its inline polygons when the sender
// drew any, else its geocodes. EMMA_IDs are kept alone when present; the
// other codes an area lists then name the same place.
type Area struct {
	Polygons []json.RawMessage // GeoJSON Polygons
	Codes    []domain.AreaCode
}

// AreaResolver turns a warning's areas into one Polygon or MultiPolygon
// covering them all. ok is false when any code is unknown; the warning is
// then skipped rather than drawn smaller than it is.
type AreaResolver interface {
	Resolve(ctx context.Context, areas []Area) (geom json.RawMessage, ok bool, err error)
}

// Source is the MeteoAlarm adapter.
type Source struct {
	base      string
	countries []Country
	client    *fetch.Client
	store     ingest.EventStore
	areas     AreaResolver
	logger    *slog.Logger
	now       func() time.Time
}

// New builds the adapter over every member feed.
func New(client *fetch.Client, store ingest.EventStore, areas AreaResolver, logger *slog.Logger) *Source {
	return &Source{
		base: FeedBase, countries: Countries, client: client, store: store,
		areas: areas, logger: logger, now: time.Now,
	}
}

// Info implements ingest.Source. Coverage is the member feeds polled, so
// those countries get MeteoAlarm's official warnings (ADR 7).
func (s *Source) Info() domain.SourceInfo {
	coverage := make([]string, 0, len(s.countries))
	for _, c := range s.countries {
		coverage = append(coverage, c.ISO2)
	}
	return domain.SourceInfo{
		Coverage:         coverage,
		OfficialWarnings: true,
		ID:               ID,
		Name:             "MeteoAlarm (EUMETNET)",
		URL:              "https://meteoalarm.org/",
		License:          "CC BY 4.0",
		Attribution:      "MeteoAlarm (EUMETNET) and the national meteorological services",
		PollInterval:     PollInterval,
		Enabled:          true,
	}
}

// Poll implements ingest.Source. A failing country is logged and skipped,
// so one member's outage does not hide every other country's warnings;
// the poll fails only when every country failed. Each country is stored
// in its own transaction: all of Europe in one would hold the stream's
// change lock long enough to delay every other source.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	now := s.now()
	var (
		superseded []string
		failures   []error
		unresolved int
		upserted   int
	)
	for _, c := range s.countries {
		var events []domain.Event
		warnings, err := s.country(ctx, c, now)
		if err != nil {
			s.logger.Warn("meteoalarm country failed", "country", c.ISO2, "error", err)
			failures = append(failures, fmt.Errorf("%s: %w", c.ISO2, err))
			continue
		}
		for i := range warnings {
			w := &warnings[i]
			superseded = append(superseded, w.Supersedes...)
			if w.Cancel {
				continue
			}
			geom, ok, err := s.areas.Resolve(ctx, w.Areas)
			if err != nil {
				return ingest.Result{}, fmt.Errorf("resolve areas: %w", err)
			}
			if !ok {
				unresolved++
				continue
			}
			w.Geometry = geom
			events = append(events, w.Event)
		}
		n, err := s.store.Upsert(ctx, ID, events)
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", c.ISO2, err)
		}
		upserted += n
	}
	if len(failures) == len(s.countries) {
		return ingest.Result{}, errors.Join(failures...)
	}
	if unresolved > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "unknown_area").Add(float64(unresolved))
		s.logger.Warn("meteoalarm warnings skipped: unknown area codes", "count", unresolved)
	}

	deleted, err := s.store.Delete(ctx, ID, superseded)
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Upserted: upserted + deleted}, nil
}

func (s *Source) country(ctx context.Context, c Country, now time.Time) ([]Warning, error) {
	resp, err := s.client.Get(ctx, s.base+c.Slug, domain.Validators{})
	if err != nil {
		return nil, err
	}
	warnings, skipped, err := Parse(resp.Body, now)
	if err != nil {
		return nil, err
	}
	for reason, n := range skipped {
		metrics.SourceSkipped.WithLabelValues(ID, reason).Add(float64(n))
	}
	for i := range warnings {
		if !warnings[i].Cancel {
			warnings[i].Attrs["country"] = c.ISO2
			warnings[i].CountryRule = domain.CountryRule{Issuer: c.ISO2, MinShare: NeighborShare}
		}
	}
	return warnings, nil
}

// Warning is one parsed alert. Geometry is left empty for Poll to resolve
// from Areas.
type Warning struct {
	domain.Event
	Areas      []Area   // in feed order
	Supersedes []string // CAP identifiers this alert replaces
	Cancel     bool     // a Cancel message: only Supersedes matters
}

type feed struct {
	Warnings []struct {
		Alert alert `json:"alert"`
	} `json:"warnings"`
}

type alert struct {
	Identifier string `json:"identifier"`
	Sent       string `json:"sent"`
	Status     string `json:"status"`
	MsgType    string `json:"msgType"`
	References string `json:"references"`
	Info       []info `json:"info"`
}

type info struct {
	Language    string      `json:"language"`
	Event       string      `json:"event"`
	Headline    string      `json:"headline"`
	Description string      `json:"description"`
	Instruction string      `json:"instruction"`
	Severity    string      `json:"severity"`
	Certainty   string      `json:"certainty"`
	Urgency     string      `json:"urgency"`
	Onset       string      `json:"onset"`
	Expires     string      `json:"expires"`
	SenderName  string      `json:"senderName"`
	Web         string      `json:"web"`
	Parameter   []valuePair `json:"parameter"`
	Area        []struct {
		AreaDesc string      `json:"areaDesc"`
		Polygon  []string    `json:"polygon"` // "lat,lon lat,lon ...", closed
		Geocode  []valuePair `json:"geocode"`
	} `json:"area"`
}

type valuePair struct {
	ValueName string `json:"valueName"`
	Value     string `json:"value"`
}

// types maps MeteoAlarm awareness types to Shield types.
var types = map[int]domain.EventType{
	1:  domain.TypeWind,
	2:  domain.TypeSnowIce,
	3:  domain.TypeStorm, // thunderstorm
	4:  domain.TypeOther, // fog
	5:  domain.TypeHeat,
	6:  domain.TypeCold,
	7:  domain.TypeOther, // coastal event: high waves, storm surge
	8:  domain.TypeWildfire,
	9:  domain.TypeSnowIce, // avalanche
	10: domain.TypeHeavyRain,
	12: domain.TypeFlood,
	13: domain.TypeFlood, // rain-flood
}

// Skip reasons for records Parse drops, as counted in
// shield_source_skipped_total. Awareness level 1 (green, "no particular
// awareness required") is dropped by design (ADR 2) and not counted: it is
// most of every feed.
const (
	SkipNoArea  = "no_area" // no area, or one we cannot draw
	SkipInvalid = "invalid" // no info block, unknown level, unparsable time
)

// Parse normalizes one country feed. now decides which warnings have
// expired. skipped counts the records dropped, by reason.
func Parse(body []byte, now time.Time) (warnings []Warning, skipped map[string]int, err error) {
	var f feed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, nil, fmt.Errorf("meteoalarm: decode: %w", err)
	}
	skipped = map[string]int{}
	var out []Warning
	for i := range f.Warnings {
		a := &f.Warnings[i].Alert
		if a.Identifier == "" || !strings.EqualFold(a.Status, "Actual") {
			continue // exercises, tests, drafts
		}
		w := Warning{Supersedes: references(a.References)}
		if strings.EqualFold(a.MsgType, "Cancel") {
			w.Cancel = true
			out = append(out, w)
			continue
		}
		in := pickInfo(a.Info)
		if in == nil {
			skipped[SkipInvalid]++
			continue
		}
		ev, areas, reason := normalize(a, in, now)
		switch reason {
		case "":
			w.Event, w.Areas = ev, areas
			out = append(out, w)
		case skipGreen:
		default:
			skipped[reason]++
		}
	}
	return out, skipped, nil
}

const skipGreen = "green"

// normalize builds the event and its areas, or names why it cannot.
func normalize(a *alert, in *info, now time.Time) (domain.Event, []Area, string) {
	levelNum, color := awareness(param(in.Parameter, "awareness_level"))
	level, ok := severity.MeteoalarmAwareness(levelNum)
	if levelNum == 1 {
		return domain.Event{}, nil, skipGreen
	}
	if !ok {
		return domain.Event{}, nil, SkipInvalid
	}
	typeNum, typeName := awareness(param(in.Parameter, "awareness_type"))
	typ, ok := types[typeNum]
	if !ok {
		typ = domain.TypeOther
	}
	sent, err1 := time.Parse(time.RFC3339, a.Sent)
	expires, err2 := time.Parse(time.RFC3339, in.Expires)
	if err1 != nil || err2 != nil {
		return domain.Event{}, nil, SkipInvalid
	}
	onset, err := time.Parse(time.RFC3339, in.Onset)
	if err != nil {
		onset = sent
	}

	var (
		areas         []Area
		descs, emmaID []string
	)
	for _, ar := range in.Area {
		descs = append(descs, ar.AreaDesc)
		area, ok := capArea(ar.Polygon, ar.Geocode)
		if !ok {
			return domain.Event{}, nil, SkipNoArea // never drawn smaller than it is
		}
		for _, c := range area.Codes {
			if c.Type == domain.AreaTypeEMMA && !slices.Contains(emmaID, c.Value) {
				emmaID = append(emmaID, c.Value)
			}
		}
		areas = append(areas, area)
	}
	if len(areas) == 0 {
		return domain.Event{}, nil, SkipNoArea
	}

	status, ended := domain.StatusActive, (*time.Time)(nil)
	if !expires.After(now) {
		e := expires.UTC()
		status, ended = domain.StatusResolved, &e
	}
	started := onset.UTC()
	title := strings.TrimSpace(in.Headline)
	if title == "" {
		title = strings.TrimSpace(in.Event)
	}
	url := in.Web
	if url == "" {
		url = "https://meteoalarm.org/"
	}
	summary := strings.TrimSpace(in.Description)
	if instr := strings.TrimSpace(in.Instruction); instr != "" {
		summary = strings.TrimSpace(summary + "\n\n" + instr)
	}

	return domain.Event{
		SourceEventID:  a.Identifier,
		Type:           typ,
		Kind:           domain.KindWarning,
		Status:         status,
		Severity:       level.String(),
		SourceSeverity: sourceSeverity(levelNum, color),
		Title:          title,
		Summary:        summary,
		StartedAt:      &started,
		ObservedAt:     sent.UTC(),
		UpdatedAt:      sent.UTC(),
		EndedAt:        ended,
		URL:            url,
		Attrs: map[string]any{
			"awarenessLevel": levelNum,
			"awarenessType":  typeName,
			"capSeverity":    in.Severity,
			"certainty":      in.Certainty,
			"urgency":        in.Urgency,
			"expiresAt":      expires.UTC(),
			"sender":         in.SenderName,
			"language":       in.Language,
			"areas":          descs,
			"emmaIds":        emmaID,
		},
	}, areas, ""
}

// capArea keeps an area's inline polygons when any parse, else its
// geocodes: the EMMA_IDs when it has some, otherwise every other code.
// ok is false when the area has neither.
func capArea(polygons []string, geocodes []valuePair) (Area, bool) {
	var a Area
	for _, p := range polygons {
		if g, ok := capPolygon(p); ok {
			a.Polygons = append(a.Polygons, g)
		}
	}
	if len(a.Polygons) > 0 {
		return a, true
	}
	var emma, other []domain.AreaCode
	for _, g := range geocodes {
		v := strings.TrimSpace(g.Value)
		if v == "" || g.ValueName == "" {
			continue
		}
		c := domain.AreaCode{Type: g.ValueName, Value: v}
		if c.Type == domain.AreaTypeEMMA {
			emma = append(emma, c)
		} else {
			other = append(other, c)
		}
	}
	a.Codes = emma
	if len(a.Codes) == 0 {
		a.Codes = other
	}
	return a, len(a.Codes) > 0
}

// capPolygon turns a CAP polygon ("lat,lon lat,lon ...", at least four
// points, first equal to last) into a GeoJSON Polygon. An open ring is
// closed; anything malformed or out of range is rejected.
func capPolygon(s string) (json.RawMessage, bool) {
	fields := strings.Fields(s)
	ring := make([][2]float64, 0, len(fields)+1)
	for _, f := range fields {
		latS, lonS, ok := strings.Cut(f, ",")
		if !ok {
			return nil, false
		}
		lat, err1 := strconv.ParseFloat(latS, 64)
		lon, err2 := strconv.ParseFloat(lonS, 64)
		if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, false
		}
		ring = append(ring, [2]float64{lon, lat})
	}
	if len(ring) > 0 && ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}
	if len(ring) < 4 {
		return nil, false
	}
	b, err := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][2]float64{ring}})
	return b, err == nil
}

// pickInfo prefers English, the language Shield's UI ships first, then the
// first block.
func pickInfo(infos []info) *info {
	for i := range infos {
		if strings.HasPrefix(strings.ToLower(infos[i].Language), "en") {
			return &infos[i]
		}
	}
	if len(infos) > 0 {
		return &infos[0]
	}
	return nil
}

func param(ps []valuePair, name string) string {
	for _, p := range ps {
		if p.ValueName == name {
			return p.Value
		}
	}
	return ""
}

// awareness splits "3; orange; Severe" or "2; snow-ice" into the number and
// the first label.
func awareness(v string) (int, string) {
	parts := strings.Split(v, ";")
	n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, ""
	}
	label := ""
	if len(parts) > 1 {
		label = strings.TrimSpace(parts[1])
	}
	return n, label
}

// references extracts identifiers from CAP's space-separated
// "sender,identifier,sent" triplets.
func references(v string) []string {
	var ids []string
	for _, ref := range strings.Fields(v) {
		parts := strings.Split(ref, ",")
		if len(parts) == 3 && parts[1] != "" {
			ids = append(ids, parts[1])
		}
	}
	return ids
}

// sourceSeverity renders MeteoAlarm's own label: "Orange warning".
func sourceSeverity(level int, color string) string {
	if color == "" {
		return fmt.Sprintf("Awareness level %d", level)
	}
	return strings.ToUpper(color[:1]) + strings.ToLower(color[1:]) + " warning"
}

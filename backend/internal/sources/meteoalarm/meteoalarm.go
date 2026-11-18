// Package meteoalarm ingests official weather warnings for Europe from
// MeteoAlarm (EUMETNET): CAP 1.2 alerts from each member's national
// weather service, one public JSON feed per country.
//
// Warning areas arrive only as EMMA_ID codes, with no geometry, so the
// adapter needs an AreaResolver to turn codes into shapes. Until one
// exists (MeteoAlarm's geocodes file, or its EDR API with a token) the
// adapter is not registered with the worker; see BACKLOG S1.5.
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

// AreaResolver turns EMMA_ID area codes into one Polygon or MultiPolygon
// covering them all. ok is false when any code is unknown; the warning is
// then skipped rather than drawn smaller than it is.
type AreaResolver interface {
	Resolve(ctx context.Context, emmaIDs []string) (geom json.RawMessage, ok bool, err error)
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
// the poll fails only when every country failed.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	now := s.now()
	var (
		events     []domain.Event
		superseded []string
		failures   []error
		unresolved int
	)
	for _, c := range s.countries {
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
	}
	if len(failures) == len(s.countries) {
		return ingest.Result{}, errors.Join(failures...)
	}
	if unresolved > 0 {
		s.logger.Warn("meteoalarm warnings skipped: unknown area codes", "count", unresolved)
	}

	upserted, err := s.store.Upsert(ctx, ID, events)
	if err != nil {
		return ingest.Result{}, err
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
	warnings, err := Parse(resp.Body, now)
	if err != nil {
		return nil, err
	}
	for i := range warnings {
		if !warnings[i].Cancel {
			warnings[i].Attrs["country"] = c.ISO2
		}
	}
	return warnings, nil
}

// Warning is one parsed alert. Geometry is left empty for Poll to resolve
// from Areas.
type Warning struct {
	domain.Event
	Areas      []string // EMMA_ID codes, deduplicated, in feed order
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

// Parse normalizes one country feed. now decides which warnings have
// expired.
func Parse(body []byte, now time.Time) ([]Warning, error) {
	var f feed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("meteoalarm: decode: %w", err)
	}
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
			continue
		}
		ev, areas, ok := normalize(a, in, now)
		if !ok {
			continue
		}
		w.Event, w.Areas = ev, areas
		out = append(out, w)
	}
	return out, nil
}

func normalize(a *alert, in *info, now time.Time) (domain.Event, []string, bool) {
	levelNum, color := awareness(param(in.Parameter, "awareness_level"))
	level, ok := severity.MeteoalarmAwareness(levelNum)
	if !ok {
		return domain.Event{}, nil, false
	}
	typeNum, typeName := awareness(param(in.Parameter, "awareness_type"))
	typ, ok := types[typeNum]
	if !ok {
		typ = domain.TypeOther
	}
	sent, err1 := time.Parse(time.RFC3339, a.Sent)
	expires, err2 := time.Parse(time.RFC3339, in.Expires)
	if err1 != nil || err2 != nil {
		return domain.Event{}, nil, false
	}
	onset, err := time.Parse(time.RFC3339, in.Onset)
	if err != nil {
		onset = sent
	}

	var areas, descs []string
	for _, ar := range in.Area {
		descs = append(descs, ar.AreaDesc)
		for _, g := range ar.Geocode {
			if g.ValueName == "EMMA_ID" && g.Value != "" && !slices.Contains(areas, g.Value) {
				areas = append(areas, g.Value)
			}
		}
	}
	if len(areas) == 0 {
		return domain.Event{}, nil, false
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
			"emmaIds":        areas,
		},
	}, areas, true
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

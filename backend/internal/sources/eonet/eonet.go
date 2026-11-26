// Package eonet ingests NASA's Earth Observatory Natural Event Tracker
// (EONET v3).
//
// Feed: /api/v3/events?status=all&days=30, every 10 minutes (ADR 16).
// status=all rather than open, so an event's closing reaches us and it is
// resolved; days=30 bounds the payload to recently active events (the open
// list alone carries thousands of never-closed records, 7,160 of them
// wildfires on 2026-11-26).
//
// days=30 filters on observation dates, and a volcano's only observation
// is the start of its eruption, so every continuing eruption fell outside
// it (all 32 open on 2026-11-26, Fuego's dating from 2002). Volcanoes are
// therefore also read from the open list (?status=open&category=volcanoes,
// about 20 KB), and a volcano that leaves it is resolved: a closing is
// never in the 30-day window either, since EONET's closed date is not an
// observation. An empty or failed open list resolves nothing.
//
// EONET aggregates other feeds. Same-origin duplicates are dropped, per
// ADR 5's ownership rule:
//   - events whose source is GDACS (Shield ingests GDACS directly),
//     except volcanoes that GVP also lists: GDACS alerts one eruption
//     episode, GVP keeps the eruption open (Fuego lists both);
//   - earthquakes (USGS owns them) and severe storms (JTWC/NHC tracks of
//     the cyclones GDACS already carries with an impact alert);
//   - categories that are not hazards to people: sea and lake ice,
//     water color, man-made.
//
// What remains is mostly incident-level wildfires (IRWIN and other
// agencies), volcanoes (Smithsonian GVP), landslides, dust and snow.
//
// License: public domain (NASA); attribution requested.
package eonet

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/severity"
)

const (
	// ID is the source key in the sources table.
	ID = "eonet"
	// FeedURL lists every event active in the last 30 days, open or closed.
	FeedURL = "https://eonet.gsfc.nasa.gov/api/v3/events?status=all&days=30"
	// VolcanoesURL lists every open volcano, whenever its eruption began.
	VolcanoesURL = "https://eonet.gsfc.nasa.gov/api/v3/events?status=open&category=volcanoes"
	// PollInterval: EONET's upstreams update from minutes to days.
	PollInterval = 10 * time.Minute
)

// Store is what the adapter needs: events, and the open volcanoes to end
// once EONET no longer lists them.
type Store interface {
	ingest.EventStore
	ActiveIDsWithAttr(ctx context.Context, sourceID, key, value string) ([]string, error)
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
}

// Source is the EONET adapter.
type Source struct {
	url        string
	volcanoURL string
	client     *fetch.Client
	store      Store
	now        func() time.Time
}

// New builds the adapter on the production feed.
func New(client *fetch.Client, store Store) *Source {
	return &Source{url: FeedURL, volcanoURL: VolcanoesURL, client: client, store: store, now: time.Now}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "NASA EONET (Earth Observatory Natural Event Tracker)",
		URL:          "https://eonet.gsfc.nasa.gov/",
		License:      "Public domain (NASA)",
		Attribution:  "NASA Earth Observatory Natural Event Tracker (EONET)",
		Scope:        "Natural events worldwide, curated by NASA from other agencies",
		TermsURL:     "https://www.earthdata.nasa.gov/engage/open-data-services-software-policies/data-use-guidance",
		Cadence:      "minutes to days, per upstream agency",
		PollInterval: PollInterval,
		Enabled:      true,
	}
}

// Poll implements ingest.Source. EONET sends no validators; the event
// store skips unchanged rows, so a quiet poll costs two downloads. Both
// lists are read before anything is written.
func (s *Source) Poll(ctx context.Context, v domain.Validators) (ingest.Result, error) {
	resp, err := s.client.Get(ctx, s.url, v)
	if err != nil {
		return ingest.Result{}, err
	}
	vresp, err := s.client.Get(ctx, s.volcanoURL, domain.Validators{})
	if err != nil {
		return ingest.Result{}, err
	}
	var events []domain.Event
	if !resp.NotModified {
		if events, err = Parse(resp.Body); err != nil {
			return ingest.Result{}, err
		}
	}
	volcanoes, err := Parse(vresp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	open, err := listed(vresp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	n, err := s.store.Upsert(ctx, ID, merge(events, volcanoes))
	if err != nil {
		return ingest.Result{}, err
	}
	r, err := s.resolveClosedVolcanoes(ctx, open)
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Validators: resp.Validators, Upserted: n + r}, nil
}

// resolveClosedVolcanoes ends the stored volcanoes the open list no longer
// carries, at the poll: EONET's own closed date (often weeks earlier) is
// not in either list. An empty list resolves nothing: 20 to 40 volcanoes
// are open at any time, so it is far likelier a fault than a quiet Earth.
func (s *Source) resolveClosedVolcanoes(ctx context.Context, open []string) (int, error) {
	if len(open) == 0 {
		return 0, nil
	}
	active, err := s.store.ActiveIDsWithAttr(ctx, ID, "category", "volcanoes")
	if err != nil {
		return 0, err
	}
	still := make(map[string]bool, len(open))
	for _, id := range open {
		still[id] = true
	}
	var gone []string
	for _, id := range active {
		if !still[id] {
			gone = append(gone, id)
		}
	}
	return s.store.Resolve(ctx, ID, gone, s.now())
}

// merge adds the open volcanoes the 30-day window did not carry.
func merge(events, volcanoes []domain.Event) []domain.Event {
	seen := make(map[string]bool, len(events))
	for i := range events {
		seen[events[i].SourceEventID] = true
	}
	for i := range volcanoes {
		if !seen[volcanoes[i].SourceEventID] {
			events = append(events, volcanoes[i])
		}
	}
	return events
}

// listed is every event id a response carries, kept or not, so a volcano
// skipped for its geometry is never resolved for being unlisted.
func listed(body []byte) ([]string, error) {
	var f struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("eonet: decode: %w", err)
	}
	ids := make([]string, 0, len(f.Events))
	for _, e := range f.Events {
		if e.ID != "" {
			ids = append(ids, e.ID)
		}
	}
	return ids, nil
}

// types maps EONET categories to Shield types. Absent categories are
// skipped.
var types = map[string]domain.EventType{
	"wildfires":    domain.TypeWildfire,
	"volcanoes":    domain.TypeVolcano,
	"floods":       domain.TypeFlood,
	"drought":      domain.TypeDrought,
	"landslides":   domain.TypeOther,
	"dustHaze":     domain.TypeOther,
	"snow":         domain.TypeSnowIce,
	"tempExtremes": domain.TypeOther, // heat or cold; EONET does not say which
}

// ownedElsewhere lists upstreams Shield ingests directly.
var ownedElsewhere = map[string]bool{"GDACS": true}

type feed struct {
	Events []struct {
		ID          string     `json:"id"`
		Title       string     `json:"title"`
		Description *string    `json:"description"`
		Link        string     `json:"link"`
		Closed      *time.Time `json:"closed"`
		Categories  []struct {
			ID string `json:"id"`
		} `json:"categories"`
		Sources  []upstream    `json:"sources"`
		Geometry []observation `json:"geometry"`
	} `json:"events"`
}

// upstream is the agency feed an EONET event came from.
type upstream struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type observation struct {
	Date      time.Time       `json:"date"`
	Type      string          `json:"type"`
	Coords    json.RawMessage `json:"coordinates"`
	Magnitude *float64        `json:"magnitudeValue"`
	Unit      *string         `json:"magnitudeUnit"`
}

// Parse normalizes an EONET v3 events response.
func Parse(body []byte) ([]domain.Event, error) {
	var f feed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("eonet: decode: %w", err)
	}
	var events []domain.Event
	for i := range f.Events {
		e := &f.Events[i]
		if e.ID == "" || len(e.Categories) == 0 || len(e.Geometry) == 0 {
			continue
		}
		typ, ok := types[e.Categories[0].ID]
		if !ok || ownedElsewhereFor(typ, e.Sources) {
			continue
		}
		// Observations come oldest first, but do not rely on it.
		obs := append([]observation(nil), e.Geometry...)
		sort.SliceStable(obs, func(a, b int) bool { return obs[a].Date.Before(obs[b].Date) })
		first, latest := obs[0], obs[len(obs)-1]
		if latest.Type != "Point" && latest.Type != "Polygon" {
			continue
		}
		geom, err := json.Marshal(map[string]any{"type": latest.Type, "coordinates": latest.Coords})
		if err != nil {
			return nil, err
		}

		level, sourceSeverity := classify(typ, latest)
		prescribed := typ == domain.TypeWildfire && isPrescribed(e.Title)
		if prescribed {
			level = severity.Minor
		}
		status, ended := domain.StatusActive, (*time.Time)(nil)
		if e.Closed != nil {
			closed := e.Closed.UTC()
			status, ended = domain.StatusResolved, &closed
		}
		started := first.Date.UTC()

		attrs := map[string]any{
			"severityProvisional": true,
			"category":            e.Categories[0].ID,
			"eonetUrl":            e.Link,
			"observations":        len(obs),
		}
		ids := make([]string, 0, len(e.Sources))
		for _, u := range e.Sources {
			ids = append(ids, u.ID)
		}
		attrs["upstream"] = ids
		if prescribed {
			attrs["prescribed"] = true
		}
		if latest.Magnitude != nil && latest.Unit != nil {
			attrs["magnitudeValue"] = *latest.Magnitude
			attrs["magnitudeUnit"] = *latest.Unit
		}

		officialURL := e.Link
		if len(e.Sources) > 0 && e.Sources[0].URL != "" {
			officialURL = e.Sources[0].URL
		}
		// A volcano's page is GVP's, whichever upstream EONET lists first
		// (Fuego's first is a 2017 Copernicus map).
		if i := slices.IndexFunc(e.Sources, func(u upstream) bool { return u.ID == "SIVolcano" && u.URL != "" }); typ == domain.TypeVolcano && i >= 0 {
			// Some arrive as http://; the site serves https.
			officialURL = strings.Replace(e.Sources[i].URL, "http://volcano.si.edu/", "https://volcano.si.edu/", 1)
		}
		summary := ""
		if e.Description != nil {
			summary = strings.TrimSpace(*e.Description)
		}

		events = append(events, domain.Event{
			SourceEventID:  e.ID,
			Type:           typ,
			Kind:           domain.KindObserved,
			Status:         status,
			Severity:       level.String(),
			SourceSeverity: sourceSeverity,
			Title:          strings.TrimSpace(e.Title),
			Summary:        summary,
			Geometry:       geom,
			StartedAt:      &started,
			ObservedAt:     latest.Date.UTC(),
			UpdatedAt:      latest.Date.UTC(),
			EndedAt:        ended,
			URL:            officialURL,
			Attrs:          attrs,
		})
	}
	return events, nil
}

// ownedElsewhereFor reports an event another Shield source carries from
// the same origin. A volcano GVP lists is GVP's, even when GDACS is among
// its upstreams: GDACS alerts single eruption episodes (none current on
// 2026-11-26), so dropping it would leave an erupting Fuego off the map.
func ownedElsewhereFor(typ domain.EventType, sources []upstream) bool {
	if typ == domain.TypeVolcano && slices.ContainsFunc(sources, func(u upstream) bool { return u.ID == "SIVolcano" }) {
		return false
	}
	return slices.ContainsFunc(sources, func(u upstream) bool { return ownedElsewhere[u.ID] })
}

// classify derives severity. EONET carries no alert level; only a
// wildfire's burned area says anything about scale, so every EONET
// severity is provisional (attrs.severityProvisional; S2.1 lifts it to a
// top-level field). The label is the area as burned: "2,379 acres burned",
// "5,000 ha burned"; other categories show the raw magnitude, or nothing.
func classify(typ domain.EventType, o observation) (severity.Level, string) {
	if o.Magnitude == nil || o.Unit == nil {
		return severity.Minor, ""
	}
	value := formatNumber(*o.Magnitude)
	if typ == domain.TypeWildfire {
		switch strings.ToLower(*o.Unit) {
		case "acres":
			return severity.WildfireAcres(*o.Magnitude), value + " acres burned"
		case "hectare", "hectares", "ha":
			return severity.WildfireAcres(*o.Magnitude * severity.AcresPerHectare), value + " ha burned"
		}
	}
	return severity.Minor, value + " " + *o.Unit
}

// formatNumber renders 2378.77 as "2,379".
func formatNumber(v float64) string {
	n := int64(math.Round(v))
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 && r != '-' && s[i-1] != '-' {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isPrescribed reports a planned burn: IRWIN names them "Prescribed Fire
// ... RX". They stay on the map, so someone who sees smoke can find the
// cause, but always as minor and flagged (owner, 2026-11-19): a fire set
// on purpose and under control is not a hazard of its size.
func isPrescribed(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	return strings.HasPrefix(t, "prescribed fire") || strings.HasPrefix(t, "prescribed burn") ||
		strings.HasSuffix(t, " rx") || strings.Contains(t, " rx ")
}

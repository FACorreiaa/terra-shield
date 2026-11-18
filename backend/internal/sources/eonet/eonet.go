// Package eonet ingests NASA's Earth Observatory Natural Event Tracker
// (EONET v3).
//
// Feed: /api/v3/events?status=all&days=30, every 10 minutes (ADR 16).
// status=all rather than open, so an event's closing reaches us and it is
// resolved; days=30 bounds the payload to recently active events (the open
// list alone carries thousands of never-closed records).
//
// EONET aggregates other feeds. Same-origin duplicates are dropped, per
// ADR 5's ownership rule:
//   - events whose source is GDACS (Shield ingests GDACS directly);
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
	// PollInterval: EONET's upstreams update from minutes to days.
	PollInterval = 10 * time.Minute
)

// Source is the EONET adapter.
type Source struct {
	url    string
	client *fetch.Client
	store  ingest.EventStore
}

// New builds the adapter on the production feed.
func New(client *fetch.Client, store ingest.EventStore) *Source {
	return &Source{url: FeedURL, client: client, store: store}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "NASA EONET (Earth Observatory Natural Event Tracker)",
		URL:          "https://eonet.gsfc.nasa.gov/",
		License:      "Public domain (NASA)",
		Attribution:  "NASA Earth Observatory Natural Event Tracker (EONET)",
		PollInterval: PollInterval,
		Enabled:      true,
	}
}

// Poll implements ingest.Source. EONET sends no validators; the event
// store skips unchanged rows, so a quiet poll costs one small download.
func (s *Source) Poll(ctx context.Context, v domain.Validators) (ingest.Result, error) {
	resp, err := s.client.Get(ctx, s.url, v)
	if err != nil {
		return ingest.Result{}, err
	}
	if resp.NotModified {
		return ingest.Result{Validators: resp.Validators, NotModified: true}, nil
	}
	events, err := Parse(resp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	n, err := s.store.Upsert(ctx, ID, events)
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Validators: resp.Validators, Upserted: n}, nil
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
		if !ok || fromOwnedSource(e.Sources) {
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
		if latest.Magnitude != nil && latest.Unit != nil {
			attrs["magnitudeValue"] = *latest.Magnitude
			attrs["magnitudeUnit"] = *latest.Unit
		}

		officialURL := e.Link
		if len(e.Sources) > 0 && e.Sources[0].URL != "" {
			officialURL = e.Sources[0].URL
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

func fromOwnedSource(sources []upstream) bool {
	for _, s := range sources {
		if ownedElsewhere[s.ID] {
			return true
		}
	}
	return false
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

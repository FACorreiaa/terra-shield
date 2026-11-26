// Package nws adapts the US National Weather Service's active alerts
// (api.weather.gov): official watches, warnings, advisories and statements
// for the states and territories. Public domain, no key (ADR 31).
//
// One stored event is one NWS hazard event, followed by its P-VTEC code
// across messages: an alert is a message about one or more events, and an
// event's area is often split over several alerts (segments). An event
// ends when the list no longer holds it in force: its segments were
// canceled, expired or upgraded, or it left the list. Never by the clock:
// a fifth of listed alerts are past their CAP expiry, still in force.
package nws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/metrics"
)

// ID is the source key.
const ID = "nws"

const (
	// URL lists every alert in force, about 700 on 2026-11-26 (3.3 MB,
	// 245 KB gzipped), with an ETag.
	URL = "https://api.weather.gov/alerts/active?status=actual"
	// PollInterval: warnings are live (ADR 16); the API caches for 5 s.
	PollInterval = time.Minute
)

// Store is where the adapter writes.
type Store interface {
	ingest.EventStore
	ActiveStarts(ctx context.Context, sourceID string) (map[string]time.Time, error)
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
}

// Drawer turns an alert's zones and polygons into one area.
type Drawer interface {
	Draw(ctx context.Context, zones []domain.ZoneRef, polygons []json.RawMessage) (json.RawMessage, []string, error)
}

// Source is the NWS adapter.
type Source struct {
	url    string
	client *fetch.Client
	store  Store
	zones  Drawer
	logger *slog.Logger
	now    func() time.Time
}

// New builds the adapter. The client must allow bodies far larger than
// today's 3.3 MB: an outbreak lists several times as many alerts.
func New(client *fetch.Client, store Store, zones Drawer, logger *slog.Logger) *Source {
	return &Source{url: URL, client: client, store: store, zones: zones, logger: logger, now: time.Now}
}

// Info implements ingest.Source.
func (s *Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		Coverage:         Coverage,
		OfficialWarnings: true,
		ID:               ID,
		Name:             "National Weather Service (NOAA)",
		URL:              "https://www.weather.gov/",
		License:          "Public domain (US government work)",
		Attribution:      "National Weather Service (NOAA)",
		Scope:            "Official weather watches, warnings and advisories for the US and its territories, over land",
		Cadence:          "as each forecast office issues them",
		PollInterval:     PollInterval,
		Enabled:          true,
	}
}

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, v domain.Validators) (ingest.Result, error) {
	resp, err := s.client.Get(ctx, s.url, v)
	if err != nil {
		return ingest.Result{}, err
	}
	if resp.NotModified {
		return ingest.Result{Validators: resp.Validators, NotModified: true}, nil
	}
	p, err := Parse(resp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	for reason, n := range p.Skipped {
		metrics.SourceSkipped.WithLabelValues(ID, reason).Add(float64(n))
	}
	if len(p.Unknown) > 0 {
		s.logger.Warn("nws phenomena not in the type table; stored as other", "codes", p.Unknown)
	}

	starts, err := s.store.ActiveStarts(ctx, ID)
	if err != nil {
		return ingest.Result{}, err
	}
	// One stored id per key: the latest year. Poll never stores two, but
	// should it happen the others are ended rather than left frozen.
	ids := slices.Sorted(maps.Keys(starts))
	stored := make(map[string]string, len(ids)) // key to stored id
	var gone []string
	for _, id := range ids {
		if old, ok := stored[keyOf(id)]; ok {
			gone = append(gone, old)
		}
		stored[keyOf(id)] = id
	}
	now := s.now().UTC()
	events := make([]domain.Event, 0, len(p.Warnings))
	undrawn, partial := 0, 0
	for i := range p.Warnings {
		w := &p.Warnings[i]
		geom, found, err := s.zones.Draw(ctx, w.Zones, w.Polygons)
		if err != nil {
			return ingest.Result{}, err
		}
		if geom == nil {
			undrawn++ // stays listed, so a stored version is kept, not ended
			continue
		}
		if len(found) < len(w.Zones) && len(w.Polygons) == 0 {
			partial++
		}
		e := w.Event
		e.Geometry = geom
		e.SourceEventID = stored[w.Key]
		// Once an event is under way its messages carry only their own
		// time as the onset: the start stored first holds.
		if began := starts[e.SourceEventID]; e.SourceEventID != "" && !began.IsZero() && !began.After(now) &&
			(e.StartedAt == nil || began.Before(*e.StartedAt)) {
			e.StartedAt = &began
		}
		if e.SourceEventID == "" {
			// Event numbers restart every year: the year Shield first saw
			// it keeps next year's number 1 apart from this year's.
			e.SourceEventID = fmt.Sprintf("%s.%d", w.Key, now.Year())
		}
		events = append(events, e)
	}
	if undrawn > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "unknown_zone").Add(float64(undrawn))
		s.logger.Warn("nws events skipped: no zone they name is loaded", "count", undrawn)
	}
	if partial > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "partial_zones").Add(float64(partial))
	}
	n, err := s.store.Upsert(ctx, ID, events)
	if err != nil {
		return ingest.Result{}, err
	}

	ended := 0
	if p.Unreadable > 0 {
		// An alert we cannot read may be about any stored event.
		metrics.SourceSkipped.WithLabelValues(ID, "unreadable").Add(float64(p.Unreadable))
		s.logger.Warn("nws alerts unreadable; no event ended this poll", "count", p.Unreadable)
	} else {
		for key, id := range stored {
			if !p.Live[key] {
				gone = append(gone, id)
			}
		}
		if ended, err = s.store.Resolve(ctx, ID, gone, now); err != nil {
			return ingest.Result{}, err
		}
	}
	return ingest.Result{Validators: resp.Validators, Upserted: n + ended}, nil
}

// keyOf strips the year from a stored id ("KPAH.WW.Y.0002.2026").
func keyOf(id string) string {
	if i := strings.LastIndexByte(id, '.'); i > 0 {
		return id[:i]
	}
	return id
}

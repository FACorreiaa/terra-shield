// Package tsunami adapts the US Tsunami Warning Centers (NOAA/NWS): the
// Pacific center in Honolulu (PTWC) and the National center in Palmer,
// Alaska (NTWC). Their messages are public domain and need no key.
//
// Two kinds of message are stored (owner, 2026-11-19):
//
//   - US products (Hawaii, Guam, American Samoa, the Marianas, Puerto Rico,
//     Alaska, the US West Coast, ...): official Warning, Advisory or Watch
//     for the NWS zones they name, drawn from NOAA's zone shapes.
//   - PTWC threat messages for the Pacific and Caribbean: forecast wave
//     heights per country, issued as guidance for national authorities.
//     Stored as forecasts at the epicenter, tagged with the countries they
//     name, never counted as official warnings (ADR 7).
//
// Each message is a snapshot of its product. A US product holds one event
// per alert level (a message can mix Warning, Advisory and Watch segments
// along different coasts), a threat message one forecast; each is updated
// bulletin by bulletin and resolved when a later message no longer holds
// it (cancellation, final message, information statement, downgrade) or
// when it expires.
package tsunami

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/metrics"
)

// ID is the source key.
const ID = "tsunami"

const (
	// Base is where messages and the list of recent ones live.
	Base = "https://www.tsunami.gov"
	// ListPath is the list of the last 40 messages of both centers, the
	// only one with every product (the Atom feeds hold the latest only).
	ListPath = "/events/js/previous.js"
	// window: older messages cannot still be in force (US products expire
	// after 3 h, threat forecasts after threatExpiry), so a restart reads
	// only the last day's.
	window = 24 * time.Hour
)

// Store is where the adapter writes.
type Store interface {
	ingest.EventStore
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
	ResolveExpired(ctx context.Context, sourceID string, now time.Time) (int, error)
}

// ZoneResolver draws NWS zones.
type ZoneResolver interface {
	Union(ctx context.Context, ugcs []string) (json.RawMessage, []string, error)
}

// Source is the tsunami adapter.
type Source struct {
	base   string
	client *fetch.Client
	store  Store
	zones  ZoneResolver
	logger *slog.Logger
	now    func() time.Time
	// done is the newest bulletin handled per product, so a poll fetches
	// only messages it has not handled. Lost on restart, when the next
	// changed list re-reads the last day's messages (their latest bulletin
	// per product, idempotent). The list's validators only advance once
	// every message in it was handled, so a 304 never hides a failed one.
	done map[string]int
}

// New builds the adapter.
func New(client *fetch.Client, store Store, zones ZoneResolver, logger *slog.Logger) *Source {
	return &Source{base: Base, client: client, store: store, zones: zones, logger: logger, now: time.Now, done: map[string]int{}}
}

// Info implements ingest.Source. Coverage is where the centers issue
// official warnings; threat forecasts elsewhere are guidance only.
func (s *Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:               ID,
		Name:             "NOAA Tsunami Warning Centers",
		URL:              "https://www.tsunami.gov",
		License:          "Public domain (US Government work)",
		Attribution:      "NOAA/NWS Pacific and National Tsunami Warning Centers",
		Scope:            "Official tsunami warnings for the US and its territories; international threat messages elsewhere, as guidance only",
		TermsURL:         "https://www.weather.gov/disclaimer",
		Cadence:          "after each earthquake that could cause a tsunami, then about hourly",
		PollInterval:     time.Minute,
		Enabled:          true,
		Coverage:         []string{"US", "AS", "GU", "MP", "PR", "VI", "UM"},
		OfficialWarnings: true,
	}
}

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, v domain.Validators) (ingest.Result, error) {
	now := s.now().UTC()
	resp, err := s.client.Get(ctx, s.base+ListPath, v)
	if err != nil {
		return ingest.Result{}, err
	}
	changed := 0
	if !resp.NotModified {
		entries, err := ParseListing(resp.Body)
		if err != nil {
			return ingest.Result{}, err
		}
		n, failed, err := s.handle(ctx, Latest(entries, now.Add(-window)), now)
		if err != nil {
			return ingest.Result{}, err
		}
		changed += n
		if failed > 0 {
			resp.Validators = v // retry the failed messages next poll
		}
	}
	expired, err := s.store.ResolveExpired(ctx, ID, now)
	if err != nil {
		return ingest.Result{}, err
	}
	changed += expired
	return ingest.Result{Validators: resp.Validators, NotModified: resp.NotModified && expired == 0, Upserted: changed}, nil
}

// handle reads each message not handled yet, oldest first, and stores
// what it means. A message that fails to fetch or parse is logged and
// retried next poll; the others still go through.
func (s *Source) handle(ctx context.Context, entries []Entry, now time.Time) (changed, failed int, err error) {
	for _, e := range entries {
		if s.done[e.Key()] >= e.Bulletin {
			continue
		}
		out, err := s.read(ctx, e)
		if err != nil {
			if ctx.Err() != nil {
				return changed, failed, ctx.Err()
			}
			failed++
			s.logger.Warn("tsunami message skipped", "key", e.Key(), "bulletin", e.Bulletin, "error", err)
			metrics.SourceSkipped.WithLabelValues(ID, "unreadable").Inc()
			continue
		}
		n, err := s.apply(ctx, e, out, now)
		if err != nil {
			return changed, failed, err // a store error: River retries the poll
		}
		changed += n
		s.done[e.Key()] = e.Bulletin
	}
	if failed > 0 && failed == len(entries) {
		return changed, failed, errors.New("tsunami: no message could be read")
	}
	return changed, failed, nil
}

func (s *Source) read(ctx context.Context, e Entry) (Outcome, error) {
	resp, err := s.client.Get(ctx, capURL(s.base, e), domain.Validators{})
	if err != nil {
		return Outcome{}, err
	}
	a, err := ParseCAP(resp.Body)
	if err != nil {
		return Outcome{}, err
	}
	out, err := Classify(e, &a)
	if err != nil || !out.NeedsText {
		return out, err
	}
	text, err := s.client.Get(ctx, textURL(s.base, e), domain.Validators{})
	if err != nil {
		return Outcome{}, fmt.Errorf("bulletin text: %w", err)
	}
	return Threat(e, &a, string(text.Body))
}

// apply stores a message's snapshot: its events, drawn, are upserted and
// the product's other events resolved. An event already past its expiry
// (a stale latest bulletin read after a restart) counts as ended, so it
// is never stored as active again.
func (s *Source) apply(ctx context.Context, e Entry, out Outcome, now time.Time) (int, error) {
	if out.Ignore {
		return 0, nil
	}
	keep := map[string]bool{}
	var events []domain.Event
	for i := range out.Events {
		d := &out.Events[i]
		ev := d.Event
		if exp, ok := ev.Attrs["expiresAt"].(time.Time); ok && !exp.After(now) {
			continue
		}
		if len(d.Zones) > 0 {
			s.draw(ctx, &ev, d.Zones)
		}
		keep[ev.SourceEventID] = true
		events = append(events, ev)
	}
	var gone []string
	for _, k := range ProductKeys(e) {
		if !keep[k] {
			gone = append(gone, k)
		}
	}
	ended, err := s.store.Resolve(ctx, ID, gone, out.At)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return ended, nil
	}
	n, err := s.store.Upsert(ctx, ID, events)
	return n + ended, err
}

// draw replaces the epicenter with the zones a US product names. Zones we
// do not hold are recorded; with none at all the event stays at the
// epicenter rather than being dropped: a tsunami warning is never hidden
// for want of a shape.
func (s *Source) draw(ctx context.Context, ev *domain.Event, zones []string) {
	geom, found, err := s.zones.Union(ctx, zones)
	if err != nil {
		s.logger.Warn("tsunami zones not drawn", "key", ev.SourceEventID, "error", err)
		return
	}
	if missing := missingZones(zones, found); len(missing) > 0 {
		ev.Attrs["zonesUnresolved"] = missing
		metrics.SourceSkipped.WithLabelValues(ID, "unknown_zone").Add(float64(len(missing)))
	}
	if geom == nil {
		return
	}
	ev.Geometry = geom
	ev.Attrs["geometry"] = "zones"
}

func missingZones(asked, found []string) []string {
	have := make(map[string]bool, len(found))
	for _, f := range found {
		have[f] = true
	}
	var out []string
	for _, a := range asked {
		if !have[a] {
			out = append(out, a)
		}
	}
	return out
}

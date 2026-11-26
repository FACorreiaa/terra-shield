// Package emsc ingests earthquakes from the EMSC (Euro-Mediterranean
// Seismological Centre) SeismicPortal, for Europe and the Mediterranean
// (ADR 34).
//
// USGS lists Europe's earthquakes only from about M4, so EMSC, which
// gathers the region's national networks, fills the M2–4 range. Where
// both report one earthquake, USGS keeps it (its PAGER alert and tsunami
// flag feed severity): an EMSC quake USGS stores within seconds,
// kilometers and tenths of magnitude is not stored, and an EMSC copy
// stored before USGS listed it (EMSC is usually minutes faster in Europe)
// is removed once USGS does.
//
// Feed: the FDSN event web service, GeoJSON, the past day in the region
// (https://www.seismicportal.eu/fdsn-wsevent.html). Like USGS, a quake the
// window no longer lists is resolved: it shows as active for its first
// day. No validators are sent: the service's ETags are not shown to follow
// its content, and fetch.Client would take a matching one as unchanged.
//
// CC BY 4.0, commercial use allowed with attribution ("EMSC-CSEM
// SeismicPortal"), terms checked 2026-11-26.
package emsc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/severity"
	"shield/internal/sources/usgs"
)

const (
	// ID is the source key in the sources table.
	ID = "emsc"
	// QueryURL is the FDSN event service.
	QueryURL = "https://www.seismicportal.eu/fdsnws/event/1/query"
	// PollInterval: EMSC publishes within minutes (it states under five);
	// two minutes keeps a quake's first report about as fresh.
	PollInterval = 2 * time.Minute
	// Window is how long a quake stays listed, and so active: a day, as
	// USGS.
	Window = 24 * time.Hour
	// MinMagnitude is the floor. M2–2.5 is about 25 quakes a day in the
	// region, M2.5 and up about 19 (2026-11-26); raise it to thin the map.
	MinMagnitude = 2.0
)

// Region is the box EMSC covers best: Europe, the Mediterranean, North
// Africa's coast, the Middle East's west and the North Atlantic to
// Iceland and the Azores.
var Region = struct{ MinLat, MaxLat, MinLon, MaxLon float64 }{27, 72, -32, 48}

// Coverage is the countries in the region: at least 90% of their area
// inside it, plus France, Norway and Morocco, whose shortfall is overseas
// territory, Svalbard and Western Sahara.
var Coverage = []string{
	"AD", "AL", "AM", "AT", "AX", "BA", "BE", "BG", "BY", "CH", "CY", "CZ", "DE", "DK", "EE", "ES",
	"FI", "FO", "FR", "GB", "GE", "GG", "GR", "HR", "HU", "IE", "IL", "IM", "IQ", "IS", "IT", "JE",
	"JO", "LB", "LI", "LT", "LU", "LV", "MA", "MC", "MD", "ME", "MK", "MT", "NL", "NO", "PL", "PS",
	"PT", "RO", "RS", "SE", "SI", "SK", "SM", "SY", "TN", "TR", "UA", "VA", "XK",
}

// SameQuake is how close a USGS report must be to be the same earthquake.
// The week sampled on 2026-11-26 paired within 4 s, 20 km and 0.2 of
// magnitude; 30 s keeps an aftershock minutes later its own event.
var SameQuake = domain.QuakeMatch{Within: 30 * time.Second, Meters: 100_000, MagDelta: 0.8}

// Store is what the adapter writes: events, the active set it compares
// the window with, and the quakes USGS already stores.
type Store interface {
	ingest.EventStore
	ActiveIDs(ctx context.Context, sourceID string) ([]string, error)
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
	KnownQuakes(ctx context.Context, sourceID string, quakes []domain.QuakeOrigin, m domain.QuakeMatch) ([]bool, error)
}

// Source is the EMSC adapter.
type Source struct {
	url    string
	client *fetch.Client
	store  Store
	now    func() time.Time
}

// New builds the adapter on the production service.
func New(client *fetch.Client, store Store) *Source {
	return &Source{url: QueryURL, client: client, store: store, now: time.Now}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "EMSC (Euro-Mediterranean Seismological Centre)",
		URL:          "https://www.seismicportal.eu/",
		License:      "CC BY 4.0",
		Attribution:  "EMSC-CSEM SeismicPortal",
		TermsURL:     "https://www.seismicportal.eu/terms.html",
		Scope:        "Earthquakes of magnitude 2 and above in Europe and the Mediterranean that USGS does not list",
		Cadence:      "within minutes",
		PollInterval: PollInterval,
		Coverage:     Coverage,
		Enabled:      true,
	}
}

// query is the past day in the region, at or above the floor.
func (s *Source) query(now time.Time) string {
	v := url.Values{}
	v.Set("format", "json")
	v.Set("starttime", now.Add(-Window).UTC().Format("2006-01-02T15:04:05"))
	v.Set("minmagnitude", strconv.FormatFloat(MinMagnitude, 'f', -1, 64))
	v.Set("minlatitude", strconv.FormatFloat(Region.MinLat, 'f', -1, 64))
	v.Set("maxlatitude", strconv.FormatFloat(Region.MaxLat, 'f', -1, 64))
	v.Set("minlongitude", strconv.FormatFloat(Region.MinLon, 'f', -1, 64))
	v.Set("maxlongitude", strconv.FormatFloat(Region.MaxLon, 'f', -1, 64))
	v.Set("orderby", "time")
	v.Set("limit", "5000")
	return s.url + "?" + v.Encode()
}

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	now := s.now().UTC()
	resp, err := s.client.Get(ctx, s.query(now), domain.Validators{})
	if err != nil {
		return ingest.Result{}, err
	}
	f, err := Parse(resp.Body) // a 204 (nothing in the window) has no body
	if err != nil {
		return ingest.Result{}, err
	}
	known, err := s.store.KnownQuakes(ctx, usgs.ID, f.origins, SameQuake)
	if err != nil {
		return ingest.Result{}, err
	}
	var keep []domain.Event
	var dup []string
	for i := range f.Events {
		if known[i] {
			dup = append(dup, f.Events[i].SourceEventID)
			continue
		}
		keep = append(keep, f.Events[i])
	}
	n, err := s.store.Upsert(ctx, ID, keep)
	if err != nil {
		return ingest.Result{}, err
	}
	// USGS has them now: an EMSC copy stored earlier goes.
	d, err := s.store.Delete(ctx, ID, dup)
	if err != nil {
		return ingest.Result{}, err
	}
	r, err := s.resolveUnlisted(ctx, f.Listed, now)
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Upserted: n + d + r}, nil
}

// resolveUnlisted ends the active quakes the window no longer lists. A
// window listing nothing resolves nothing: the region has dozens of M2+
// quakes a day (328 in the week to 2026-11-26), so an empty one is far
// likelier a fault than a quiet day.
func (s *Source) resolveUnlisted(ctx context.Context, listed []string, now time.Time) (int, error) {
	if len(listed) == 0 {
		return 0, nil
	}
	active, err := s.store.ActiveIDs(ctx, ID)
	if err != nil {
		return 0, err
	}
	listed = slices.Clone(listed)
	slices.Sort(listed)
	var gone []string
	for _, id := range active {
		if _, found := slices.BinarySearch(listed, id); !found {
			gone = append(gone, id)
		}
	}
	return s.store.Resolve(ctx, ID, gone, now)
}

type feed struct {
	Features []struct {
		ID         string `json:"id"`
		Properties struct {
			SourceID   string   `json:"source_id"` // the EMSC website's id
			LastUpdate string   `json:"lastupdate"`
			Time       string   `json:"time"`
			Region     string   `json:"flynn_region"`
			Lat        *float64 `json:"lat"`
			Lon        *float64 `json:"lon"`
			Depth      *float64 `json:"depth"` // km, positive down
			EvType     string   `json:"evtype"`
			Auth       string   `json:"auth"` // the agency that located it
			Mag        *float64 `json:"mag"`
			MagType    string   `json:"magtype"`
		} `json:"properties"`
	} `json:"features"`
}

// Feed is one parsed window.
type Feed struct {
	// Events are the quakes to store, before the USGS check.
	Events []domain.Event
	// origins pairs with Events, for that check.
	origins []domain.QuakeOrigin
	// Listed is every id the window carries, stored or not, so only a
	// quake that left the window is resolved.
	Listed []string
}

// Parse normalizes an FDSN event GeoJSON answer; an empty body (204) is
// an empty window.
//
// Only earthquakes are kept: EMSC's event type codes end in "e" for one
// (ke known, se suspected, fe felt, ue no certainty; QuakeML maps all four
// to earthquake), and anything else (a blast, an explosion) is not a
// hazard. A quake without magnitude, position or time is listed but not
// stored until EMSC completes it.
func Parse(body []byte) (Feed, error) {
	var out Feed
	if len(strings.TrimSpace(string(body))) == 0 {
		return out, nil
	}
	var f feed
	if err := json.Unmarshal(body, &f); err != nil {
		return out, fmt.Errorf("emsc: decode: %w", err)
	}
	for i := range f.Features {
		ft := &f.Features[i]
		p := &ft.Properties
		if ft.ID == "" || !strings.HasSuffix(p.EvType, "e") {
			continue
		}
		out.Listed = append(out.Listed, ft.ID)
		origin, err := time.Parse(time.RFC3339, p.Time)
		if err != nil || p.Mag == nil || p.Lat == nil || p.Lon == nil {
			continue
		}
		origin = origin.UTC()
		updated, err := time.Parse(time.RFC3339, p.LastUpdate)
		if err != nil {
			updated = origin
		}
		mag, lat, lon := *p.Mag, *p.Lat, *p.Lon
		place := regionName(p.Region)
		attrs := map[string]any{
			"mag":     mag,
			"magType": p.MagType,
			"place":   place,
		}
		if p.Depth != nil {
			attrs["depthKm"] = *p.Depth
		}
		if p.Auth != "" {
			attrs["agency"] = p.Auth
		}
		ev := domain.Event{
			SourceEventID:  ft.ID,
			Type:           domain.TypeEarthquake,
			Kind:           domain.KindObserved,
			Status:         domain.StatusActive,
			Severity:       severity.Earthquake(mag, "", false).String(),
			SourceSeverity: strings.TrimSpace(fmt.Sprintf("M%.1f %s", mag, p.MagType)),
			Title:          strings.TrimSuffix(fmt.Sprintf("M %.1f - %s", mag, place), " - "),
			Summary:        place,
			Geometry:       domain.PointGeometry(lon, lat),
			StartedAt:      &origin,
			ObservedAt:     origin,
			UpdatedAt:      updated.UTC(),
			Attrs:          attrs,
		}
		if p.SourceID != "" {
			ev.URL = "https://www.emsc-csem.org/Earthquake_information/earthquake.php?id=" + url.QueryEscape(p.SourceID)
		}
		out.Events = append(out.Events, ev)
		out.origins = append(out.origins, domain.QuakeOrigin{At: origin, Lon: lon, Lat: lat, Mag: mag})
	}
	return out, nil
}

// regionName turns a Flinn-Engdahl region ("DODECANESE ISLANDS, GREECE",
// "TURKEY-IRAN BORDER REGION") into a title ("Dodecanese Islands,
// Greece", "Turkey-Iran Border Region"); "of" and "and" stay lower case.
func regionName(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if i > 0 && (w == "of" || w == "and") {
			continue
		}
		r := []rune(w)
		for j := range r {
			if j == 0 || r[j-1] == '-' || r[j-1] == '(' || r[j-1] == '.' {
				r[j] = unicode.ToUpper(r[j])
			}
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

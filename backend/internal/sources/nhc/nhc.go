// Package nhc adapts the official tropical cyclone forecasts of NOAA's
// National Hurricane Center and Central Pacific Hurricane Center: each
// storm's forecast cone and track for the Atlantic and the eastern and
// central North Pacific. Public domain, no key (ADR 32).
//
// CurrentStorms.json lists every storm advised on, with the latest public
// advisory's position and intensity and a link to the latest forecast's
// GIS archive (cone and track points). One stored event is one storm,
// kind forecast: the cone is its geometry and its pin is the storm's
// center. A storm ends when the list no longer holds it.
package nhc

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/metrics"
	"shield/internal/severity"
)

// ID is the source key.
const ID = "nhc"

const (
	// URL lists the storms advised on; 24 bytes when there are none, about
	// 4 KB per storm, with an ETag.
	URL = "https://www.nhc.noaa.gov/CurrentStorms.json"
	// PollInterval: advisories come every 3 to 6 hours per storm, at
	// times that shift with intermediate and special advisories.
	PollInterval = 2 * time.Minute
	// host is the only host whose archives are followed.
	host = "www.nhc.noaa.gov"
)

// Store is where the adapter writes.
type Store interface {
	ingest.EventStore
	ActiveStarts(ctx context.Context, sourceID string) (map[string]time.Time, error)
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
}

// Source is the NHC adapter.
type Source struct {
	url     string
	client  *fetch.Client
	store   Store
	logger  *slog.Logger
	now     func() time.Time
	allowed func(*url.URL) bool // which archive links are followed

	mu       sync.Mutex
	archives map[string]*forecast // by archive URL: one per advisory, never changed
}

// New builds the adapter.
func New(client *fetch.Client, store Store, logger *slog.Logger) *Source {
	return &Source{
		url: URL, client: client, store: store, logger: logger, now: time.Now,
		allowed:  func(u *url.URL) bool { return u.Scheme == "https" && u.Host == host },
		archives: map[string]*forecast{},
	}
}

// Info implements ingest.Source.
func (s *Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "National Hurricane Center (NOAA)",
		URL:          "https://www.nhc.noaa.gov/",
		License:      "Public domain (US government work)",
		Attribution:  "NOAA National Hurricane Center and Central Pacific Hurricane Center",
		TermsURL:     "https://www.weather.gov/disclaimer",
		Scope:        "Official tropical cyclone forecasts (cone and track) for the Atlantic and the eastern and central North Pacific",
		Cadence:      "every 6 hours per storm; every 3 hours under watches or warnings",
		PollInterval: PollInterval,
		Enabled:      true,
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
	storms, err := parseCurrent(resp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	stored, err := s.store.ActiveStarts(ctx, ID)
	if err != nil {
		return ingest.Result{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	keep := make(map[string]*forecast, len(storms))
	listed := make(map[string]bool, len(storms))
	events := make([]domain.Event, 0, len(storms))
	pending := false
	for i := range storms {
		st := &storms[i]
		listed[st.ID] = true
		f, link, err := s.forecast(ctx, st)
		switch {
		case err != nil:
			// Usually the list updated before the archive was published.
			// The stored event stays as it was, and the next poll asks again.
			pending = true
			metrics.SourceSkipped.WithLabelValues(ID, "forecast_unavailable").Inc()
			s.logger.Warn("nhc forecast unavailable; storm kept as stored", "storm", st.ID, "err", err)
			continue
		case f == nil:
			metrics.SourceSkipped.WithLabelValues(ID, "no_forecast").Inc()
			continue // no cone (a final advisory, a remnant low): nothing to draw
		}
		keep[link] = f
		e, err := build(st, f)
		if err != nil {
			metrics.SourceSkipped.WithLabelValues(ID, "unreadable").Inc()
			s.logger.Warn("nhc storm unreadable; kept as stored", "storm", st.ID, "err", err)
			continue
		}
		events = append(events, e)
	}
	s.archives = keep

	n, err := s.store.Upsert(ctx, ID, events)
	if err != nil {
		return ingest.Result{}, err
	}
	var gone []string
	for id := range stored {
		if !listed[id] {
			gone = append(gone, id)
		}
	}
	sort.Strings(gone)
	ended, err := s.store.Resolve(ctx, ID, gone, s.now().UTC())
	if err != nil {
		return ingest.Result{}, err
	}
	validators := resp.Validators
	if pending {
		validators = domain.Validators{} // an unchanged list must not hide the missing archive
	}
	return ingest.Result{Validators: validators, Upserted: n + ended}, nil
}

// forecast returns the storm's latest forecast and its archive link; nil
// without error when NHC publishes no cone for it.
func (s *Source) forecast(ctx context.Context, st *storm) (*forecast, string, error) {
	if st.TrackCone == nil || st.TrackCone.ZipFile == "" {
		return nil, "", nil
	}
	link := st.TrackCone.ZipFile
	u, err := url.Parse(link)
	if err != nil || !s.allowed(u) {
		return nil, "", fmt.Errorf("archive link %q is not NHC's", link)
	}
	if f, ok := s.archives[link]; ok {
		return f, link, nil
	}
	when, err := issued(st.TrackCone.Issuance)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.client.Get(ctx, link, domain.Validators{})
	if err != nil {
		return nil, "", err
	}
	f, err := parseForecast(resp.Body, when)
	if err != nil {
		return nil, "", err
	}
	return f, link, nil
}

// build makes the storm's event. The figures and the pin come from the
// latest public advisory, intermediate ones included; the cone and track
// from the latest forecast, which only full advisories update.
func build(st *storm, f *forecast) (domain.Event, error) {
	updated, err := issued(st.LastUpdate)
	if err != nil && st.PublicAdvisory != nil {
		updated, err = issued(st.PublicAdvisory.Issuance)
	}
	if err != nil {
		return domain.Event{}, err
	}
	coneIssued, err := issued(st.TrackCone.Issuance)
	if err != nil {
		return domain.Event{}, err
	}

	first := f.Points[0]
	pos := st.position()
	if pos == nil {
		pos = &[2]float64{first.Lon, first.Lat}
	}
	now, ok := st.knots()
	if !ok {
		now = first.WindKt
	}
	var peak float64
	for _, p := range f.Points {
		peak = math.Max(peak, p.WindKt)
	}

	attrs := map[string]any{
		"stormId":          st.ID,
		"basin":            st.basin(),
		"classification":   strings.ToUpper(st.Classification),
		"forecastAdvisory": st.TrackCone.AdvNum,
		"forecastIssuedAt": coneIssued.Format(time.RFC3339),
		"position":         []float64{pos[0], pos[1]},
		"peakWindKt":       peak,
		"forecast":         track(f.Points),
	}
	if st.PublicAdvisory != nil && st.PublicAdvisory.AdvNum != "" {
		attrs["advisory"] = st.PublicAdvisory.AdvNum
	}
	if now > 0 {
		attrs["windKt"] = now
	}
	if mb, ok := st.pressure(); ok {
		attrs["pressureMb"] = mb
	}
	if st.MovementDir != nil && *st.MovementDir >= 0 && *st.MovementDir <= 360 {
		attrs["movementDeg"] = *st.MovementDir
	}
	if st.MovementSpeed != nil && *st.MovementSpeed >= 0 && *st.MovementSpeed < missing {
		attrs["movementMph"] = *st.MovementSpeed
	}

	return domain.Event{
		SourceEventID:  st.ID,
		Type:           domain.TypeTropicalCyclone,
		Kind:           domain.KindForecast,
		Status:         domain.StatusActive,
		Severity:       severity.CycloneWindKt(math.Max(now, peak)).String(),
		SourceSeverity: sourceSeverity(now, peak),
		Title:          st.title(),
		Summary:        summary(now, attrs, peak),
		Geometry:       f.Cone,
		Position:       pos,
		ObservedAt:     updated,
		UpdatedAt:      updated,
		URL:            officialPage(st),
		Attrs:          attrs,
	}, nil
}

// track is the forecast points for attrs, without unknown values.
func track(points []point) []map[string]any {
	out := make([]map[string]any, 0, len(points))
	for _, p := range points {
		m := map[string]any{"tau": p.Tau, "lon": p.Lon, "lat": p.Lat}
		if !p.At.IsZero() {
			m["at"] = p.At.Format(time.RFC3339)
		}
		if p.WindKt > 0 {
			m["windKt"] = p.WindKt
		}
		if p.GustKt > 0 {
			m["gustKt"] = p.GustKt
		}
		if p.Pressure > 0 {
			m["pressureMb"] = p.Pressure
		}
		if p.Type != "" {
			m["stormType"] = p.Type
		}
		out = append(out, m)
	}
	return out
}

// kmh converts knots for display; rating uses knots (CycloneWindKt).
func kmh(kt float64) float64 { return math.Round(kt * 1.852) }

// category is the Saffir-Simpson category of a wind in knots; 0 below
// hurricane force.
func category(kt float64) int {
	switch {
	case kt >= 137:
		return 5
	case kt >= 113:
		return 4
	case kt >= 96:
		return 3
	case kt >= 83:
		return 2
	case kt >= 64:
		return 1
	}
	return 0
}

func windLabel(kt float64) string {
	if c := category(kt); c > 0 {
		return fmt.Sprintf("Cat %d", c)
	}
	return fmt.Sprintf("%.0f kt", kt)
}

// sourceSeverity is NHC's intensity now and, when stronger, at its
// forecast peak: "Cat 3 now, Cat 4 forecast". Empty when NHC gave none.
func sourceSeverity(now, peak float64) string {
	if now <= 0 && peak <= 0 {
		return ""
	}
	if now <= 0 {
		return windLabel(peak) + " forecast"
	}
	label := windLabel(now)
	if peak > now && windLabel(peak) != label {
		return label + " now, " + windLabel(peak) + " forecast"
	}
	return label
}

func summary(now float64, attrs map[string]any, peak float64) string {
	var parts []string
	if now > 0 {
		parts = append(parts, fmt.Sprintf("Maximum sustained winds %.0f kt (%.0f km/h)", now, kmh(now)))
	}
	if mb, ok := attrs["pressureMb"].(float64); ok {
		parts = append(parts, fmt.Sprintf("central pressure %.0f mb", mb))
	}
	if deg, ok := attrs["movementDeg"].(float64); ok {
		if mph, ok := attrs["movementMph"].(float64); ok {
			parts = append(parts, fmt.Sprintf("moving toward %.0f° at %.0f mph", deg, mph))
		}
	}
	s := strings.Join(parts, "; ")
	if peak > now {
		if s != "" {
			s += ". "
		}
		s += fmt.Sprintf("Forecast to peak at %.0f kt (%.0f km/h)", peak, kmh(peak))
	}
	if s != "" {
		s += "."
	}
	return s
}

// officialPage is the storm's graphics page (cone and warnings), else its
// public advisory; only NHC's own pages.
func officialPage(st *storm) string {
	for _, p := range []*product{st.ForecastGraphics, st.PublicAdvisory} {
		if p == nil || p.URL == "" {
			continue
		}
		if u, err := url.Parse(p.URL); err == nil && u.Scheme == "https" && u.Host == host {
			return p.URL
		}
	}
	return "https://www.nhc.noaa.gov/"
}

package countries

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

type fakeRepo struct {
	list    []domain.CountryStats
	summary map[string]domain.CountrySummary
	err     error
	since   time.Time
}

func (f *fakeRepo) List(context.Context) ([]domain.CountryStats, error) { return f.list, f.err }

func (f *fakeRepo) Summary(_ context.Context, iso2 string, since time.Time) (domain.CountrySummary, error) {
	f.since = since
	if f.err != nil {
		return domain.CountrySummary{}, f.err
	}
	s, ok := f.summary[iso2]
	if !ok {
		return s, domain.ErrNotFound
	}
	return s, nil
}

func get(t *testing.T, r Repository, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	f := New(r)
	f.now = func() time.Time { return now }
	srv, err := server.New(d, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	return w
}

func pt() domain.CountryStats {
	return domain.CountryStats{ISO2: "PT", ISO3: "PRT", Name: "Portugal", ActiveEvents: 2, MaxSeverity: "severe", BBox: [4]float64{-9.5, 37, -6.2, 42.1}}
}

func TestList(t *testing.T) {
	quiet := domain.CountryStats{ISO2: "IS", ISO3: "ISL", Name: "Iceland"}
	w := get(t, &fakeRepo{list: []domain.CountryStats{quiet, pt()}}, "/v1/countries", nil)
	if w.Code != http.StatusOK || w.Header().Get("ETag") == "" || w.Header().Get("Cache-Control") != "public, max-age=15" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	var body struct {
		Countries []map[string]any `json:"countries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Countries) != 2 {
		t.Fatalf("%s err %v", w.Body, err)
	}
	if _, ok := body.Countries[0]["maxSeverity"]; ok || body.Countries[0]["activeEvents"] != 0.0 {
		t.Errorf("quiet country %v", body.Countries[0])
	}
	if p := body.Countries[1]; p["maxSeverity"] != "severe" || p["iso3"] != "PRT" || len(p["bbox"].([]any)) != 4 {
		t.Errorf("PT %v", p)
	}
	if w := get(t, &fakeRepo{list: []domain.CountryStats{quiet, pt()}}, "/v1/countries", http.Header{"If-None-Match": {w.Header().Get("ETag")}}); w.Code != http.StatusNotModified {
		t.Errorf("revalidate %d", w.Code)
	}
	if w := get(t, &fakeRepo{err: errors.New("db down")}, "/v1/countries", nil); w.Code != http.StatusInternalServerError {
		t.Errorf("store error %d", w.Code)
	}
}

func TestSummary(t *testing.T) {
	ev := domain.EventRecord{
		Event: domain.Event{
			SourceEventID: "q", Type: domain.TypeEarthquake, Kind: domain.KindObserved, Status: domain.StatusActive,
			Severity: "severe", Title: "Quake", ObservedAt: now, UpdatedAt: now, Attrs: map[string]any{},
		},
		ID: "6f1c1f5e-8a54-4a0e-9b5e-2f0c3c7c1a11", SourceID: "usgs", FetchedAt: now,
	}
	repo := &fakeRepo{summary: map[string]domain.CountrySummary{
		"PT": {
			Country:  pt(),
			Counts:   []domain.SeverityCount{{Type: domain.TypeEarthquake, Severity: "severe", Count: 1}},
			Coverage: domain.Coverage{Sources: []string{"gdacs", "usgs"}},
			Events:   []domain.EventRecord{ev},
			RecentChanges: []domain.EventChange{{
				EventID: ev.ID, Title: "Quake",
				Revision: domain.Revision{At: now, Changes: map[string]any{"severity": map[string]any{"from": "minor", "to": "severe"}}},
			}},
		},
		"IS": {Country: domain.CountryStats{ISO2: "IS", ISO3: "ISL", Name: "Iceland"}},
	}}

	w := get(t, repo, "/v1/countries/pt/summary", nil)
	if w.Code != http.StatusOK || !repo.since.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("%d since %s: %s", w.Code, repo.since, w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["country"].(map[string]any)["iso2"] != "PT" || body["generatedAt"] != "2026-11-16T12:00:00Z" {
		t.Errorf("country/generatedAt %v", body)
	}
	cov := body["coverage"].(map[string]any)
	if cov["officialWarnings"] != false || len(cov["sources"].([]any)) != 2 {
		t.Errorf("coverage %v", cov)
	}
	if e := body["events"].([]any)[0].(map[string]any); e["id"] != ev.ID || e["source"] != "usgs" || e["countries"] == nil {
		t.Errorf("event %v", e)
	}
	if c := body["recentChanges"].([]any)[0].(map[string]any); c["eventId"] != ev.ID || c["changes"] == nil {
		t.Errorf("change %v", c)
	}
	if n := body["counts"].([]any)[0].(map[string]any); n["type"] != "earthquake" || n["count"] != 1.0 {
		t.Errorf("counts %v", n)
	}

	// A quiet country serializes empty arrays, never null.
	w = get(t, repo, "/v1/countries/IS/summary", nil)
	var quiet map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &quiet); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"counts", "events", "recentChanges"} {
		if a, ok := quiet[k].([]any); !ok || len(a) != 0 {
			t.Errorf("%s = %v, want []", k, quiet[k])
		}
	}
	if s, ok := quiet["coverage"].(map[string]any)["sources"].([]any); !ok || len(s) != 0 {
		t.Errorf("coverage sources = %v, want []", quiet["coverage"])
	}

	if w := get(t, repo, "/v1/countries/PRT/summary", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad iso2 %d", w.Code)
	}
	if w := get(t, repo, "/v1/countries/ZZ/summary", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown %d", w.Code)
	}
	if w := get(t, nil, "/v1/countries/PT/summary", nil); w.Code != http.StatusServiceUnavailable {
		t.Errorf("no database %d", w.Code)
	}
}

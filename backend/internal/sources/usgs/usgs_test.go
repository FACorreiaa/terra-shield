package usgs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func byID(events []domain.Event) map[string]domain.Event {
	m := map[string]domain.Event{}
	for i := range events {
		m[events[i].SourceEventID] = events[i]
	}
	return m
}

// TestParseRecordedFeed runs the parser over a real feed recorded on
// 2026-11-16, so a USGS format change shows up as a fixture refresh diff.
func TestParseRecordedFeed(t *testing.T) {
	f, err := Parse(read(t, "2.5_day.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	events, deleted := f.Events, f.Deleted
	if len(f.Listed) != len(events) {
		t.Errorf("listed %d, stored %d: every recorded quake has a magnitude", len(f.Listed), len(events))
	}
	if len(events) != 35 || len(deleted) != 0 {
		t.Fatalf("%d events, %d deleted; want 35, 0", len(events), len(deleted))
	}
	for i := range events {
		e := &events[i]
		if e.Type != domain.TypeEarthquake || e.Kind != domain.KindObserved || e.Severity == "" || e.Title == "" || e.URL == "" {
			t.Fatalf("incomplete event %+v", e)
		}
		if e.UpdatedAt.Before(e.ObservedAt) {
			t.Fatalf("%s updated before origin", e.SourceEventID)
		}
		var g struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(e.Geometry, &g); err != nil || g.Type != "Point" || len(g.Coordinates) != 2 {
			t.Fatalf("%s geometry %s", e.SourceEventID, e.Geometry)
		}
	}
	sample := byID(events)["us7000tykc"]
	if sample.Summary != "11 km SSE of Stonewall Gap, Colorado" || sample.Severity != "minor" ||
		!strings.HasPrefix(sample.SourceSeverity, "M2.8") {
		t.Fatalf("sample %+v", sample)
	}
}

func TestParseEdgeCases(t *testing.T) {
	f, err := Parse(read(t, "cases.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	events, deleted := f.Events, f.Deleted
	// The magnitude-less quake is listed though not stored; the blast and
	// the retraction are neither.
	if got := strings.Join(f.Listed, ","); got != "us_red,us_yellow_small,us_nomag,us_plain" {
		t.Errorf("listed %s", got)
	}
	m := byID(events)
	if len(events) != 3 {
		t.Fatalf("kept %d events, want 3 (red, yellow, plain)", len(events))
	}
	if _, ok := m["us_blast"]; ok {
		t.Error("quarry blast kept")
	}
	if _, ok := m["us_nomag"]; ok {
		t.Error("event without magnitude kept")
	}
	if len(deleted) != 1 || deleted[0] != "us_deleted" {
		t.Errorf("deleted %v", deleted)
	}

	red := m["us_red"]
	if red.Severity != "extreme" || red.SourceSeverity != "M7.4 mww, PAGER red, tsunami flag" {
		t.Errorf("red %q / %q", red.Severity, red.SourceSeverity)
	}
	if red.Attrs["depthKm"] != 24.5 || red.Attrs["feltReports"] != 1200 || red.Attrs["tsunami"] != true {
		t.Errorf("red attrs %v", red.Attrs)
	}
	if !red.ObservedAt.Equal(time.UnixMilli(1794840000000)) || !red.UpdatedAt.Equal(time.UnixMilli(1794843600000)) {
		t.Errorf("red times %s %s", red.ObservedAt, red.UpdatedAt)
	}

	yellow := m["us_yellow_small"]
	if yellow.Severity != "moderate" {
		t.Errorf("PAGER yellow on M4.6 = %q, want moderate", yellow.Severity)
	}
	if yellow.Title != "M 4.6 - Somewhere dense" {
		t.Errorf("fallback title %q", yellow.Title)
	}
	if _, ok := yellow.Attrs["feltReports"]; ok {
		t.Error("null felt became an attr")
	}

	if m["us_plain"].Severity != "moderate" || m["us_plain"].SourceSeverity != "M5.2 mww" {
		t.Errorf("plain %+v", m["us_plain"])
	}
}

type fakeStore struct {
	upserted   []domain.Event
	deleted    []string
	active     []string
	resolved   []string
	resolvedAt time.Time
}

func (f *fakeStore) ActiveIDs(context.Context, string) ([]string, error) { return f.active, nil }

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, at time.Time) (int, error) {
	f.resolved, f.resolvedAt = ids, at
	return len(ids), nil
}

func (f *fakeStore) Upsert(_ context.Context, sourceID string, events []domain.Event) (int, error) {
	if sourceID != ID {
		panic(sourceID)
	}
	f.upserted = events
	return len(events), nil
}

func (f *fakeStore) Delete(_ context.Context, _ string, ids []string) (int, error) {
	f.deleted = ids
	return len(ids), nil
}

func TestPoll(t *testing.T) {
	const lm = "Mon, 16 Nov 2026 16:25:47 GMT"
	body := read(t, "cases.geojson")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Last-Modified", lm) // USGS ignores conditional GET
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// us_old left the day window; us_nomag is listed without a magnitude
	// and stays; us_red is in the feed.
	store := &fakeStore{active: []string{"us_red", "us_old", "us_nomag"}}
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	s := &Source{url: srv.URL, client: fetch.New(fetch.Options{HostGap: time.Millisecond}), store: store, now: func() time.Time { return now }}

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if res.NotModified || res.Upserted != 5 || res.Validators.LastModified != lm {
		t.Fatalf("first poll %+v", res)
	}
	if len(store.upserted) != 3 || len(store.deleted) != 1 {
		t.Fatalf("store got %d upserts, %d deletes", len(store.upserted), len(store.deleted))
	}
	if !slices.Equal(store.resolved, []string{"us_old"}) || !store.resolvedAt.Equal(now) {
		t.Fatalf("resolved %v at %v", store.resolved, store.resolvedAt)
	}

	// Same Last-Modified next minute: no parse, no store call.
	store.upserted = nil
	res, err = s.Poll(t.Context(), res.Validators)
	if err != nil || !res.NotModified || store.upserted != nil {
		t.Fatalf("second poll %+v err %v, store %v", res, err, store.upserted)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits %d", hits.Load())
	}
}

// A window listing nothing is a fault: it resolves nothing.
func TestPollEmptyFeedResolvesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer srv.Close()
	store := &fakeStore{active: []string{"us_a", "us_b"}}
	s := &Source{url: srv.URL, client: fetch.New(fetch.Options{HostGap: time.Millisecond}), store: store, now: time.Now}
	if _, err := s.Poll(t.Context(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if store.resolved != nil {
		t.Fatalf("empty feed resolved %v", store.resolved)
	}
}

var _ Store = (*fakeStore)(nil)

func TestInfo(t *testing.T) {
	info := (&Source{}).Info()
	if info.ID != ID || info.PollInterval != time.Minute || !info.Enabled || info.License == "" || info.Attribution == "" {
		t.Fatalf("%+v", info)
	}
}

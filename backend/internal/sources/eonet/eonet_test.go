package eonet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
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

// TestParseRecordedFeed runs over events cut from the real feed of
// 2026-11-16: storms, sea ice and GDACS-sourced fires are all dropped,
// leaving the IRWIN wildfires.
func TestParseRecordedFeed(t *testing.T) {
	events, err := Parse(read(t, "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("%d events, want the 4 IRWIN wildfires", len(events))
	}
	for i := range events {
		e := &events[i]
		if e.Type != domain.TypeWildfire || e.Title == "" || e.URL == "" || e.SourceSeverity == "" || e.Attrs["severityProvisional"] != true {
			t.Fatalf("incomplete %+v", e)
		}
		if e.Attrs["upstream"].([]string)[0] != "IRWIN" {
			t.Fatalf("upstream %v", e.Attrs["upstream"])
		}
	}
	if byID(events)["EONET_26256"].Attrs["prescribed"] != true { // "Prescribed Fire Compartment 22 RX"
		t.Error("recorded prescribed burn not flagged")
	}
	ms := byID(events)["EONET_26465"]
	if ms.Title != "Wildfire Range 47, Forrest, Mississippi" || ms.SourceSeverity != "2,379 acres burned" || ms.Severity != "moderate" ||
		ms.URL != "https://irwin.doi.gov/observer/incidents/2026-MSMNF-000723" {
		t.Fatalf("Mississippi fire %+v", ms)
	}
}

func TestParseEdgeCases(t *testing.T) {
	events, err := Parse(read(t, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := byID(events)
	if len(events) != 5 {
		t.Fatalf("kept %d, want V1 W1 RX1 W2 L1", len(events))
	}

	// A prescribed burn stays, always minor and flagged, whatever its size.
	if rx := m["EONET_RX1"]; rx.Severity != "minor" || rx.SourceSeverity != "2,400 acres burned" || rx.Attrs["prescribed"] != true {
		t.Errorf("prescribed burn %s %q prescribed=%v", rx.Severity, rx.SourceSeverity, rx.Attrs["prescribed"])
	}
	if m["EONET_W1"].Attrs["prescribed"] != nil {
		t.Error("wildfire flagged as prescribed")
	}

	v := m["EONET_V1"]
	if v.Type != domain.TypeVolcano || v.Severity != "minor" || v.SourceSeverity != "" || v.Summary != "" ||
		v.URL != "https://volcano.si.edu/volcano.cfm?vn=211060" {
		t.Errorf("volcano %+v", v)
	}

	// Observations arrive out of order: the latest one wins, the earliest
	// starts the event.
	w1 := m["EONET_W1"]
	if w1.Severity != "extreme" || w1.SourceSeverity != "150,000 acres burned" ||
		!w1.ObservedAt.Equal(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)) ||
		!w1.StartedAt.Equal(time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("W1 %+v", w1)
	}
	var g struct {
		Coordinates [2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(w1.Geometry, &g); err != nil || g.Coordinates != [2]float64{-120.1, 38.1} {
		t.Errorf("W1 geometry %s", w1.Geometry)
	}

	// Hectares convert: 5,000 ha ≈ 12,355 acres → severe. Closed → resolved.
	w2 := m["EONET_W2"]
	if w2.Severity != "severe" || w2.SourceSeverity != "5,000 ha burned" || w2.Status != domain.StatusResolved || w2.EndedAt == nil || !json.Valid(w2.Geometry) {
		t.Errorf("W2 %+v", w2)
	}

	if l := m["EONET_L1"]; l.Type != domain.TypeOther || l.URL != "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_L1" {
		t.Errorf("landslide %+v", l)
	}
}

func TestFormatNumber(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 999.4: "999", 2378.77: "2,379", 1234567: "1,234,567"} {
		if got := formatNumber(in); got != want {
			t.Errorf("%v = %q, want %q", in, got, want)
		}
	}
}

type fakeStore struct {
	got      []domain.Event
	active   []string // stored active volcanoes
	asked    string   // the attr ActiveIDsWithAttr was asked for
	resolved []string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.got = events
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ActiveIDsWithAttr(_ context.Context, _, key, value string) ([]string, error) {
	f.asked = key + "=" + value
	return f.active, nil
}

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	f.resolved = append(f.resolved, ids...)
	return len(ids), nil
}

var _ Store = (*fakeStore)(nil)

// newSource serves the 30-day window and the open-volcano list from
// stubs; a nil volcano body answers 500.
func newSource(t *testing.T, window, volcanoes []byte, store Store) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/volcanoes") && volcanoes == nil:
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasSuffix(r.URL.Path, "/volcanoes"):
			_, _ = w.Write(volcanoes)
		default:
			_, _ = w.Write(window)
		}
	}))
	t.Cleanup(srv.Close)
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.url, s.volcanoURL = srv.URL+"/all", srv.URL+"/volcanoes"
	return s
}

func TestPoll(t *testing.T) {
	store := &fakeStore{}
	s := newSource(t, read(t, "cases.json"), []byte(`{"events": []}`), store)
	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.Upserted != 5 || len(store.got) != 5 {
		t.Fatalf("%+v err %v", res, err)
	}
	if info := s.Info(); info.ID != ID || info.PollInterval != 10*time.Minute || !info.Enabled {
		t.Fatalf("%+v", info)
	}
}

func TestIsPrescribed(t *testing.T) {
	for title, want := range map[string]bool{
		"Prescribed Fire Freidland-Chimney Piles RX, Coconino, Arizona": true,
		"Prescribed Fire Bienville CPT 29 Sub 2 RX, Scott, Mississippi": true,
		"Prescribed burn Block 4, Ocala, Florida":                       true,
		"Unit 12 RX": true,
		"Wildfire Range 47, Forrest, Mississippi": false,
		"RX Ranch Fire, Texas":                    false, // a place, not the RX suffix
	} {
		if got := isPrescribed(title); got != want {
			t.Errorf("%q = %v, want %v", title, got, want)
		}
	}
}

// The open list recorded on 2026-11-26: all 32 volcanoes kept, Fuego too
// though GDACS is among its upstreams, each dated by its eruption's start
// and linked to its GVP page.
func TestParseOpenVolcanoes(t *testing.T) {
	events, err := Parse(read(t, "volcanoes-open.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 32 {
		t.Fatalf("%d volcanoes, want 32", len(events))
	}
	for i := range events {
		if e := &events[i]; e.Type != domain.TypeVolcano || e.Status != domain.StatusActive || !strings.HasPrefix(e.URL, "https://volcano.si.edu/") {
			t.Fatalf("%s: %s %s %s", e.SourceEventID, e.Type, e.Status, e.URL)
		}
	}
	fuego := byID(events)["EONET_980"]
	if !fuego.StartedAt.Equal(time.Date(2002, 1, 4, 0, 0, 0, 0, time.UTC)) || !fuego.ObservedAt.Equal(*fuego.StartedAt) {
		t.Fatalf("Fuego dated %v / %v: its only observation is the eruption's start", fuego.StartedAt, fuego.ObservedAt)
	}
}

// A volcano only GDACS carries stays GDACS's.
func TestParseGDACSOnlyVolcano(t *testing.T) {
	body := `{"events": [{"id": "EONET_G", "title": "Etna", "categories": [{"id": "volcanoes"}],
	  "sources": [{"id": "GDACS", "url": "https://www.gdacs.org/"}],
	  "geometry": [{"date": "2026-11-20T00:00:00Z", "type": "Point", "coordinates": [15.0, 37.7]}]}]}`
	events, err := Parse([]byte(body))
	if err != nil || len(events) != 0 {
		t.Fatalf("kept %d, %v", len(events), err)
	}
}

// Open volcanoes join the window; one stored but no longer open resolves,
// and only volcanoes are asked about, so wildfires are never ended here.
func TestPollVolcanoes(t *testing.T) {
	store := &fakeStore{active: []string{"EONET_980", "EONET_CLOSED"}}
	s := newSource(t, read(t, "cases.json"), read(t, "volcanoes-open.json"), store)
	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 5+32 || res.Upserted != 5+32+1 {
		t.Fatalf("upserted %d (result %d), want 37 and one resolve", len(store.got), res.Upserted)
	}
	if store.asked != "category=volcanoes" || !slices.Equal(store.resolved, []string{"EONET_CLOSED"}) {
		t.Fatalf("asked %q, resolved %v", store.asked, store.resolved)
	}
}

// An empty open list resolves nothing; a failed one fails the poll before
// anything is written.
func TestPollVolcanoListFaults(t *testing.T) {
	store := &fakeStore{active: []string{"EONET_980"}}
	if _, err := newSource(t, read(t, "cases.json"), []byte(`{"events": []}`), store).Poll(t.Context(), domain.Validators{}); err != nil || len(store.resolved) != 0 {
		t.Fatalf("empty list: resolved %v, %v", store.resolved, err)
	}
	store = &fakeStore{active: []string{"EONET_980"}}
	if _, err := newSource(t, read(t, "cases.json"), nil, store).Poll(t.Context(), domain.Validators{}); err == nil || store.got != nil || len(store.resolved) != 0 {
		t.Fatalf("failed list: err %v, wrote %d, resolved %v", err, len(store.got), store.resolved)
	}
}

// A volcano in both lists is stored once.
func TestMerge(t *testing.T) {
	a := []domain.Event{{SourceEventID: "x"}, {SourceEventID: "y"}}
	got := merge(a, []domain.Event{{SourceEventID: "y"}, {SourceEventID: "z"}})
	ids := make([]string, len(got))
	for i := range got {
		ids[i] = got[i].SourceEventID
	}
	if !slices.Equal(ids, []string{"x", "y", "z"}) {
		t.Fatalf("%v", ids)
	}
}

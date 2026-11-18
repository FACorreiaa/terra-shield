package eonet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
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
	if len(events) != 4 {
		t.Fatalf("kept %d, want V1 W1 W2 L1", len(events))
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

type fakeStore struct{ got []domain.Event }

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.got = events
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

var _ ingest.EventStore = (*fakeStore)(nil)

func TestPoll(t *testing.T) {
	body := read(t, "cases.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()
	store := &fakeStore{}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.url = srv.URL

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.Upserted != 4 || len(store.got) != 4 {
		t.Fatalf("%+v err %v", res, err)
	}
	if info := s.Info(); info.ID != ID || info.PollInterval != 10*time.Minute || !info.Enabled {
		t.Fatalf("%+v", info)
	}
}

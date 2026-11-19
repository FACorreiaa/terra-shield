package gdacs

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
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

func byID(items []Item) map[string]domain.Event {
	m := map[string]domain.Event{}
	for i := range items {
		m[items[i].SourceEventID] = items[i].Event
	}
	return m
}

// TestParseRecordedFeed runs over 13 items cut from the real feed of
// 2026-11-16 (one per type × alert × current, plus every Orange item).
func TestParseRecordedFeed(t *testing.T) {
	events, err := Parse(read(t, "rss.xml"))
	if err != nil {
		t.Fatal(err)
	}
	m := byID(events)
	if len(events) != 12 {
		t.Fatalf("%d events, want 12 (13 items minus one EQ)", len(events))
	}
	if _, ok := m["EQ1577487"]; ok {
		t.Fatal("GDACS earthquake kept; USGS owns earthquakes")
	}
	for i := range events {
		e := &events[i]
		if e.Title == "" || e.Summary == "" || !strings.HasPrefix(e.URL, "https://www.gdacs.org/") || e.Kind != domain.KindObserved {
			t.Fatalf("incomplete %+v", e)
		}
		if (e.Status == domain.StatusResolved) != (e.EndedAt != nil) {
			t.Fatalf("%s status %s ended %v", e.SourceEventID, e.Status, e.EndedAt)
		}
	}

	tc := m["TC1001350"]
	if tc.Type != "tropical_cyclone" || tc.Title != "Tropical cyclone XAVIER-26" || tc.Severity != "moderate" || tc.Status != "active" ||
		tc.Attrs["severityBasis"] != "wind" { // Green alert, but 176 km/h is Category 2
		t.Errorf("TC %+v", tc)
	}
	if tc.SourceSeverity != "Green alert, Tropical Storm (maximum wind speed of 176 km/h)" {
		t.Errorf("TC source severity %q", tc.SourceSeverity)
	}
	if tc.Attrs["severityUnit"] != "km/h" || tc.Attrs["vulnerability"] != "Medium" {
		t.Errorf("TC attrs %v", tc.Attrs)
	}
	// GDACS bbox is "lonmin lonmax latmin latmax"; attrs hold GeoJSON order.
	if bb := tc.Attrs["bbox"].([]float64); bb[0] > bb[2] || bb[1] > bb[3] {
		t.Errorf("bbox order %v", bb)
	}

	// The burned-area table floors a Green fire: 5,437 ha is 13,435 acres.
	if wf := m["WF1033884"]; wf.Severity != "severe" || wf.Attrs["severityBasis"] != "area" ||
		wf.Summary != "On 15/11/2026, a forest fire started in Australia, until 16/11/2026." {
		t.Errorf("WF %s basis %v summary %q", wf.Severity, wf.Attrs["severityBasis"], wf.Summary)
	}
	// A depression below hurricane force keeps its alert; floods carry no magnitude.
	if td := m["TC1001348"]; td.Severity != "minor" || td.Attrs["severityBasis"] != "alert" {
		t.Errorf("depression %s basis %v", td.Severity, td.Attrs["severityBasis"])
	}
	if fl := m["FL1104285"]; fl.SourceSeverity != "Green alert" {
		t.Errorf("flood source severity %q", fl.SourceSeverity)
	}

	dr := m["DR1023877"]
	if dr.Type != "drought" || dr.Severity != "severe" || dr.Status != "active" ||
		dr.Attrs["severityBasis"] != "alert" || strings.Contains(dr.Summary, "  ") {
		t.Errorf("current Orange drought %+v", dr)
	}
	if n := len(dr.Attrs["countries"].([]string)); n < 2 {
		t.Errorf("drought countries %v", dr.Attrs["countries"])
	}

	old := m["DR1027450"]
	if old.Status != "resolved" || old.Title != "Drought: East Africa, Central Sahel-2026" ||
		!old.EndedAt.Equal(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)) ||
		!old.UpdatedAt.Equal(time.Date(2026, 11, 16, 5, 59, 18, 0, time.UTC)) {
		t.Errorf("resolved drought %+v", old)
	}
}

func TestParseEdgeCases(t *testing.T) {
	events, err := Parse(read(t, "cases.xml"))
	if err != nil {
		t.Fatal(err)
	}
	m := byID(events)
	if len(events) != 2 {
		t.Fatalf("kept %v, want VO9001 and WF9002", reflect.ValueOf(m).MapKeys())
	}

	vo := m["VO9001"]
	if vo.Type != "volcano" || vo.Severity != "extreme" || vo.Title != "Volcanic activity Merapi" || vo.SourceSeverity != "Red alert" {
		t.Errorf("VO %+v", vo)
	}
	if !reflect.DeepEqual(vo.Attrs["bbox"], []float64{108.4, -9.5, 112.4, -5.5}) {
		t.Errorf("bbox %v", vo.Attrs["bbox"])
	}

	wf := m["WF9002"]
	if wf.Severity != "moderate" || wf.Attrs["severityBasis"] != "area" { // 1,200 ha = 2,965 acres
		t.Errorf("WF9002 %s basis %v", wf.Severity, wf.Attrs["severityBasis"])
	}
	if wf.Title != "Wildfire in Brazil, Bolivia and 2 more" {
		t.Errorf("title %q", wf.Title)
	}
	if !reflect.DeepEqual(wf.Attrs["countries"], []string{"Brazil", "Bolivia", "Paraguay", "Argentina"}) {
		t.Errorf("countries %v", wf.Attrs["countries"])
	}
	// Unparseable todate: observed falls back to datemodified.
	if !wf.ObservedAt.Equal(time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("observed %s", wf.ObservedAt)
	}
}

func TestTitle(t *testing.T) {
	cases := []struct {
		typ       domain.EventType
		name      string
		countries []string
		want      string
	}{
		{domain.TypeFlood, "", []string{"Myanmar"}, "Flood in Myanmar"},
		{domain.TypeFlood, "", nil, "Flood"},
		{domain.TypeWildfire, "", []string{"A", "B"}, "Wildfire in A, B"},
		{domain.TypeDrought, "Sahel-2026", []string{"A", "B", "C"}, "Drought: Sahel-2026"},
	}
	for _, c := range cases {
		if got := title(c.typ, c.name, c.countries); got != c.want {
			t.Errorf("%v = %q, want %q", c, got, c.want)
		}
	}
}

type fakeStore struct{ got []domain.Event }

func (f *fakeStore) Upsert(_ context.Context, sourceID string, events []domain.Event) (int, error) {
	if sourceID != ID {
		panic(sourceID)
	}
	f.got = events
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

var _ ingest.EventStore = (*fakeStore)(nil)

func TestPollConditional(t *testing.T) {
	body := read(t, "rss.xml")
	geometry := read(t, "geometry_wf.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geometry" {
			_, _ = w.Write(geometry)
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	store := &fakeStore{}
	s := newTestSource(srv.URL, srv.URL+"/geometry", store)

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.Upserted != 12 || res.Validators.ETag != `"v1"` {
		t.Fatalf("first %+v %v", res, err)
	}
	store.got = nil
	res, err = s.Poll(t.Context(), res.Validators)
	if err != nil || !res.NotModified || store.got != nil {
		t.Fatalf("second %+v %v", res, err)
	}
}

func TestInfo(t *testing.T) {
	info := (&Source{}).Info()
	if info.ID != ID || info.PollInterval != 5*time.Minute || !strings.Contains(info.License, "CC BY 4.0") {
		t.Fatalf("%+v", info)
	}
}

func newTestSource(feedURL, geometryURL string, store ingest.EventStore) *Source {
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, slog.New(slog.DiscardHandler))
	s.feedURL, s.geometryURL = feedURL, geometryURL
	return s
}

func TestSelectFootprint(t *testing.T) {
	tc, ok, err := selectFootprint("TC", read(t, "geometry_tc.json"))
	if err != nil || !ok {
		t.Fatalf("TC ok=%v err %v", ok, err)
	}
	// The whole-track 60 km/h buffer, not a per-timestep one.
	var gc geometryCollection
	_ = json.Unmarshal(read(t, "geometry_tc.json"), &gc)
	for _, f := range gc.Features {
		if string(f.Geometry) == string(tc) && f.Properties.Label != "60 km/h" {
			t.Fatalf("picked %q", f.Properties.Label)
		}
	}

	if _, ok, err := selectFootprint("WF", read(t, "geometry_wf.json")); err != nil || !ok {
		t.Fatalf("WF ok=%v err %v", ok, err)
	}
	if _, ok, _ := selectFootprint("FL", read(t, "geometry_wf.json")); ok {
		t.Fatal("FL matched a WF collection")
	}
	if _, _, err := selectFootprint("WF", []byte("<html>")); err == nil {
		t.Fatal("garbage accepted")
	}
}

// The current Orange drought (DR1023877) gets its footprint; Green and
// resolved events keep their points; a second poll reuses the cache.
func TestPollAppliesFootprints(t *testing.T) {
	feed, geometry := read(t, "rss.xml"), read(t, "geometry_wf.json")
	var geometryHits atomic.Int32
	fail := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geometry" {
			geometryHits.Add(1)
			if fail.Load() {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write(geometry)
			return
		}
		_, _ = w.Write(feed) // no validators: every poll parses
	}))
	defer srv.Close()
	store := &fakeStore{}
	s := newTestSource(srv.URL, srv.URL+"/geometry", store)

	for range 2 {
		if _, err := s.Poll(t.Context(), domain.Validators{}); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]domain.Event{}
	for i := range store.got {
		got[store.got[i].SourceEventID] = store.got[i]
	}
	if !isPolygonal(got["DR1023877"].Geometry) || got["DR1023877"].Attrs["footprint"] != true {
		t.Fatalf("Orange drought geometry %s", got["DR1023877"].Geometry)
	}
	for _, id := range []string{"TC1001350", "DR1027450"} { // Green; resolved Orange
		if isPolygonal(got[id].Geometry) {
			t.Fatalf("%s got a footprint", id)
		}
	}
	if geometryHits.Load() != 1 {
		t.Fatalf("geometry fetched %d times, want once (cached per episode)", geometryHits.Load())
	}

	// A failing geometry endpoint keeps the point and does not fail the poll.
	s.footprints = newFootprintCache()
	fail.Store(true)
	if _, err := s.Poll(t.Context(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	for i := range store.got {
		if store.got[i].SourceEventID == "DR1023877" && isPolygonal(store.got[i].Geometry) {
			t.Fatal("footprint without a geometry response")
		}
	}
}

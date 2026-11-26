package emsc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/sources/usgs"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestParseRecordedFeed runs the parser over a real answer recorded on
// 2026-11-26 (the region, M3+, the week to 19:00 UTC), so a format change
// shows up as a fixture refresh diff.
func TestParseRecordedFeed(t *testing.T) {
	f, err := Parse(read(t, "emsc-week-m3.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != 51 || len(f.Listed) != 51 || len(f.origins) != 51 {
		t.Fatalf("%d events, %d listed, %d origins; want 51", len(f.Events), len(f.Listed), len(f.origins))
	}
	for i := range f.Events {
		e := &f.Events[i]
		if e.Type != domain.TypeEarthquake || e.Kind != domain.KindObserved || e.Severity == "" ||
			!strings.HasPrefix(e.Title, "M ") || !strings.HasPrefix(e.URL, "https://www.emsc-csem.org/Earthquake_information/earthquake.php?id=") {
			t.Fatalf("incomplete event %+v", e)
		}
		if e.UpdatedAt.Before(e.ObservedAt) || e.StartedAt == nil || !e.StartedAt.Equal(e.ObservedAt) {
			t.Fatalf("%s times: origin %v, updated %v", e.SourceEventID, e.ObservedAt, e.UpdatedAt)
		}
		if d, ok := e.Attrs["depthKm"].(float64); !ok || d < 0 {
			t.Fatalf("%s depth %v: want positive km", e.SourceEventID, e.Attrs["depthKm"])
		}
		if f.origins[i].Mag != e.Attrs["mag"] || !f.origins[i].At.Equal(e.ObservedAt) {
			t.Fatalf("%s origin %+v does not pair with its event", e.SourceEventID, f.origins[i])
		}
	}
	greece := f.Events[slices.IndexFunc(f.Events, func(e domain.Event) bool { return e.SourceEventID == "20261125_0000295" })]
	var g struct {
		Coordinates []float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(greece.Geometry, &g); err != nil {
		t.Fatal(err)
	}
	if greece.Title != "M 4.4 - Greece" || greece.SourceSeverity != "M4.4 mb" || greece.Severity != "minor" ||
		len(g.Coordinates) != 2 || g.Coordinates[0] != 20.68 || g.Coordinates[1] != 38.27 || greece.Attrs["agency"] != "EMSC" {
		t.Fatalf("Greece M4.4: %+v at %v", greece, g.Coordinates)
	}
}

func TestParseEdgeCases(t *testing.T) {
	for _, empty := range []string{"", " \n"} {
		f, err := Parse([]byte(empty))
		if err != nil || len(f.Events) != 0 || len(f.Listed) != 0 {
			t.Fatalf("empty body (a 204): %+v, %v", f, err)
		}
	}
	if _, err := Parse([]byte("<html>")); err == nil {
		t.Fatal("garbage must fail")
	}
	body := `{"features": [
	 {"id": "a", "properties": {"evtype": "qb", "time": "2026-11-26T10:00:00Z", "lat": 40, "lon": 20, "mag": 2.1, "magtype": "ml"}},
	 {"id": "b", "properties": {"evtype": "se", "time": "2026-11-26T10:00:00Z", "lat": 40, "lon": 20, "mag": 2.1, "magtype": "ml", "flynn_region": "NEAR COAST OF WESTERN TURKEY"}},
	 {"id": "c", "properties": {"evtype": "ke", "time": "2026-11-26T10:00:00Z", "lat": 40, "lon": 20, "magtype": "ml"}},
	 {"id": "", "properties": {"evtype": "ke", "time": "2026-11-26T10:00:00Z", "lat": 40, "lon": 20, "mag": 2.1}}
	]}`
	f, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// The blast is out; the magnitude-less quake is listed, not stored.
	if got := strings.Join(f.Listed, ","); got != "b,c" || len(f.Events) != 1 || f.Events[0].SourceEventID != "b" {
		t.Fatalf("listed %s, stored %d", got, len(f.Events))
	}
	if e := f.Events[0]; e.URL != "" || e.Title != "M 2.1 - Near Coast of Western Turkey" {
		t.Fatalf("suspected quake %+v", e)
	}
}

func TestRegionName(t *testing.T) {
	for in, want := range map[string]string{
		"DODECANESE ISLANDS, GREECE": "Dodecanese Islands, Greece",
		"TURKEY-IRAN BORDER REGION":  "Turkey-Iran Border Region",
		"STRAIT OF GIBRALTAR":        "Strait of Gibraltar",
		"":                           "",
	} {
		if got := regionName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

type fakeStore struct {
	known    []time.Time // origins USGS has
	active   []string
	upserted []domain.Event
	deleted  []string
	resolved []string
	order    []string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, es []domain.Event) (int, error) {
	f.order = append(f.order, "upsert")
	f.upserted = append(f.upserted, es...)
	return len(es), nil
}

func (f *fakeStore) Delete(_ context.Context, _ string, ids []string) (int, error) {
	f.order = append(f.order, "delete")
	f.deleted = append(f.deleted, ids...)
	return len(ids), nil
}

func (f *fakeStore) ActiveIDs(context.Context, string) ([]string, error) { return f.active, nil }

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	f.resolved = append(f.resolved, ids...)
	return len(ids), nil
}

// KnownQuakes answers by origin time: the test names the quakes USGS has.
func (f *fakeStore) KnownQuakes(_ context.Context, source string, qs []domain.QuakeOrigin, m domain.QuakeMatch) ([]bool, error) {
	if source != usgs.ID || m != SameQuake {
		return nil, nil
	}
	out := make([]bool, len(qs))
	for i, q := range qs {
		out[i] = slices.ContainsFunc(f.known, q.At.Equal)
	}
	return out, nil
}

func newSource(t *testing.T, h http.HandlerFunc, st Store) (*Source, *[]*url.URL) {
	t.Helper()
	var seen []*url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL)
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
			t.Error("validators sent")
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), st)
	s.url = srv.URL + "/fdsnws/event/1/query"
	s.now = func() time.Time { return time.Date(2026, 11, 26, 19, 0, 0, 0, time.UTC) }
	return s, &seen
}

func TestPoll(t *testing.T) {
	body := read(t, "emsc-week-m3.json")
	f, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	// USGS lists the Ithaca M4.4 (stored here before it did).
	st := &fakeStore{active: []string{"gone-from-window", "20261125_0000295"}}
	for i := range f.Events {
		if f.Events[i].SourceEventID == "20261125_0000295" {
			st.known = append(st.known, f.origins[i].At)
		}
	}
	s, seen := newSource(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `W/"1"`)
		_, _ = w.Write(body)
	}, st)
	res, err := s.Poll(t.Context(), domain.Validators{ETag: `W/"1"`})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.upserted) != 50 || !slices.Equal(st.deleted, []string{"20261125_0000295"}) {
		t.Fatalf("upserted %d, deleted %v: want 50 and the USGS duplicate", len(st.upserted), st.deleted)
	}
	if slices.ContainsFunc(st.upserted, func(e domain.Event) bool { return e.SourceEventID == "20261125_0000295" }) {
		t.Fatal("the USGS duplicate was stored")
	}
	if !slices.Equal(st.resolved, []string{"gone-from-window"}) || res.Upserted != 50+1+1 || res.NotModified {
		t.Fatalf("resolved %v, result %+v", st.resolved, res)
	}
	if got := strings.Join(st.order, ","); got != "upsert,delete" {
		t.Fatalf("store calls %s", got)
	}
	q := (*seen)[0].Query()
	if q.Get("format") != "json" || q.Get("starttime") != "2026-11-25T19:00:00" || q.Get("minmagnitude") != "2" ||
		q.Get("minlatitude") != "27" || q.Get("maxlongitude") != "48" {
		t.Fatalf("query %s", (*seen)[0].RawQuery)
	}
}

// Nothing in the window is a 204 with no body: nothing stored, nothing
// resolved.
func TestPollNoContent(t *testing.T) {
	st := &fakeStore{active: []string{"a"}}
	s, _ := newSource(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, st)
	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.Upserted != 0 || len(st.resolved) != 0 {
		t.Fatalf("result %+v, resolved %v, %v", res, st.resolved, err)
	}
}

func TestPollFails(t *testing.T) {
	s, _ := newSource(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, &fakeStore{})
	if _, err := s.Poll(t.Context(), domain.Validators{}); err == nil {
		t.Fatal("a 503 must fail the poll")
	}
}

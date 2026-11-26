package usgsvolcano

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/severity"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The lists recorded on 2026-11-26: 161 volcanoes, three above normal.
func TestParseRecordedFeeds(t *testing.T) {
	f, err := Parse(read(t, "geojson.json"), read(t, "elevated.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != 3 || len(f.Quiet) != 158 || f.Quiet["usgs:ak102"] != (time.Time{}) || f.Unreadable != 0 {
		t.Fatalf("%d events, %d quiet, %d unreadable", len(f.Events), len(f.Quiet), f.Unreadable)
	}
	by := map[string]domain.Event{}
	for _, e := range f.Events {
		by[e.SourceEventID] = e
	}
	sitkin := by["311120"]
	if sitkin.Severity != "severe" || sitkin.SourceSeverity != "Watch · aviation Orange" || sitkin.Kind != domain.KindWarning ||
		sitkin.Title != "Great Sitkin, Aleutians" || !strings.HasPrefix(sitkin.Summary, "AVO Great Sitkin ORANGE/WATCH") ||
		sitkin.URL != "https://volcanoes.usgs.gov/hans2/view/notice/DOI-USGS-AVO-2026-11-20T18:41:17+00:00" ||
		!sitkin.UpdatedAt.Equal(time.Date(2026, 11, 20, 20, 33, 25, 0, time.UTC)) {
		t.Fatalf("Great Sitkin %+v", sitkin)
	}
	ahyi := by["284141"]
	if ahyi.Severity != "moderate" || ahyi.StartedAt == nil || !ahyi.StartedAt.Equal(time.Date(2026, 7, 16, 0, 13, 50, 0, time.UTC)) ||
		ahyi.Attrs["alertLevelPrev"] != "UNASSIGNED" || ahyi.Attrs["vnum"] != "284141" {
		t.Fatalf("Ahyi %+v", ahyi)
	}
	if k := by["332010"]; k.Severity != "moderate" || k.Title != "Kilauea, Hawaii" || string(k.Geometry) != `{"coordinates":[-155.287,19.421],"type":"Point"}` {
		t.Fatalf("Kilauea %+v %s", k, k.Geometry)
	}
	if at, ok := f.Quiet["311800"]; !ok || !at.IsZero() { // Adagdak: UNASSIGNED, never a notice
		t.Fatalf("Adagdak quiet %v %v", at, ok)
	}
}

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		level, color string
		want         severity.Level
		elevated     bool
	}{
		{"ADVISORY", "YELLOW", severity.Moderate, true},
		{"WATCH", "ORANGE", severity.Severe, true},
		{"WARNING", "RED", severity.Extreme, true},
		{"WATCH", "RED", severity.Severe, true},         // ground decides
		{"UNASSIGNED", "ORANGE", severity.Severe, true}, // no ground level: aviation
		{"NORMAL", "YELLOW", 0, false},
		{"NORMAL", "GREEN", 0, false},
		{"UNASSIGNED", "UNASSIGNED", 0, false},
	} {
		got, elevated := Classify(c.level, c.color)
		if got != c.want || elevated != c.elevated {
			t.Errorf("%s/%s = %v %v, want %v %v", c.level, c.color, got, elevated, c.want, c.elevated)
		}
	}
	if s := sourceSeverity("UNASSIGNED", "ORANGE"); s != "aviation Orange" {
		t.Errorf("seamount source severity %q", s)
	}
}

func TestParseFaults(t *testing.T) {
	if _, err := Parse([]byte(`{"features": []}`), []byte(`[]`)); err == nil {
		t.Error("an empty status list must fail the poll")
	}
	if _, err := Parse([]byte(`{"features": [{}]}`), []byte(`{`)); err == nil {
		t.Error("an unreadable elevated list must fail the poll")
	}
	f, err := Parse([]byte(`{"features": [
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"vnum": "", "volcanoCd": "", "alertLevel": "WATCH", "alertDate": "2026-11-26 10:00:00"}},
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"vnum": "1", "alertLevel": "WATCH"}}
	]}`), []byte(`[]`))
	if err != nil || len(f.Events) != 0 || f.Unreadable != 2 {
		t.Fatalf("%d events, %d unreadable, %v", len(f.Events), f.Unreadable, err)
	}
}

type fakeStore struct {
	got      []domain.Event
	active   []string
	resolved map[string]time.Time
	order    []string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.order = append(f.order, "upsert")
	f.got = events
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ActiveIDs(context.Context, string) ([]string, error) { return f.active, nil }

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, at time.Time) (int, error) {
	f.order = append(f.order, "resolve")
	if f.resolved == nil {
		f.resolved = map[string]time.Time{}
	}
	for _, id := range ids {
		f.resolved[id] = at
	}
	return len(ids), nil
}

var _ Store = (*fakeStore)(nil)

func newSource(t *testing.T, status, elevated []byte, st Store) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := status
		if strings.HasSuffix(r.URL.Path, "/elevated") {
			body = elevated
		}
		if body == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), st, nil)
	s.statusURL, s.elevatedURL = srv.URL+"/geojson", srv.URL+"/elevated"
	s.now = func() time.Time { return time.Date(2026, 11, 26, 20, 0, 0, 0, time.UTC) }
	return s
}

// A stored volcano the list shows back at normal ends at its notice (or at
// the poll when it has none); one still elevated or not listed stays.
func TestPoll(t *testing.T) {
	status := read(t, "geojson.json") // Mount St. Helens at NORMAL since a 2026-10-15 notice
	st := &fakeStore{active: []string{"332010", "311800", "321050", "999999"}}
	res, err := newSource(t, status, read(t, "elevated.json"), st).Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.got) != 3 || res.Upserted != 3+2 {
		t.Fatalf("upserted %d, result %d", len(st.got), res.Upserted)
	}
	if len(st.resolved) != 2 || !st.resolved["311800"].Equal(time.Date(2026, 11, 26, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("resolved %v", st.resolved)
	}
	if at, ok := st.resolved["321050"]; !ok || !at.Equal(time.Date(2026, 10, 15, 20, 1, 3, 0, time.UTC)) {
		t.Fatalf("St. Helens resolved at %v (%v), want its notice", at, ok)
	}
	if st.order[0] != "upsert" {
		t.Fatalf("order %v", st.order)
	}
}

// Both feeds are read before anything is written.
func TestPollFails(t *testing.T) {
	st := &fakeStore{active: []string{"311800"}}
	if _, err := newSource(t, read(t, "geojson.json"), nil, st).Poll(t.Context(), domain.Validators{}); err == nil || st.got != nil || st.resolved != nil {
		t.Fatalf("err %v, wrote %d, resolved %v", err, len(st.got), st.resolved)
	}
	if info := New(nil, nil, nil).Info(); info.ID != ID || !info.OfficialWarnings || !slices.Equal(info.Coverage, []string{"US", "MP", "AS"}) {
		t.Fatalf("%+v", info)
	}
}

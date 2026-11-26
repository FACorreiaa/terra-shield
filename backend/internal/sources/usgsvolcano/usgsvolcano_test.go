package usgsvolcano

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/severity"
)

var now = time.Date(2026, 11, 26, 20, 0, 0, 0, time.UTC)

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

// The lists recorded on 2026-11-26: 161 volcanoes, three above normal.
func TestParseRecordedFeeds(t *testing.T) {
	f, err := Parse(read(t, "geojson.json"), read(t, "elevated.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != 3 || len(f.Quiet) != 158 || len(f.Listed) != 161 || f.Unreadable != 0 {
		t.Fatalf("%d events, %d quiet, %d listed, %d unreadable", len(f.Events), len(f.Quiet), len(f.Listed), f.Unreadable)
	}
	by := byID(f.Events)
	sitkin := by["ak111"]
	if sitkin.Severity != "severe" || sitkin.SourceSeverity != "Watch · aviation Orange" || sitkin.Kind != domain.KindWarning ||
		sitkin.Title != "Great Sitkin, Aleutians" || !strings.HasPrefix(sitkin.Summary, "AVO Great Sitkin ORANGE/WATCH") ||
		sitkin.URL != "https://volcanoes.usgs.gov/hans2/view/notice/DOI-USGS-AVO-2026-11-20T18:41:17+00:00" ||
		!sitkin.UpdatedAt.Equal(time.Date(2026, 11, 20, 20, 33, 25, 0, time.UTC)) || sitkin.Attrs["vnum"] != "311120" {
		t.Fatalf("Great Sitkin %+v", sitkin)
	}
	ahyi := by["nmi2"]
	if ahyi.Severity != "moderate" || ahyi.StartedAt == nil || !ahyi.StartedAt.Equal(time.Date(2026, 7, 16, 0, 13, 50, 0, time.UTC)) ||
		ahyi.Attrs["alertLevelPrev"] != "UNASSIGNED" {
		t.Fatalf("Ahyi %+v", ahyi)
	}
	if k := by["hi3"]; k.Severity != "moderate" || k.Title != "Kilauea, Hawaii" || string(k.Geometry) != `{"coordinates":[-155.287,19.421],"type":"Point"}` {
		t.Fatalf("Kilauea %+v %s", k, k.Geometry)
	}
	if at, ok := f.Quiet["ak102"]; !ok || !at.IsZero() { // Folsoms Bluff: no GVP number, never a notice
		t.Fatalf("Folsoms Bluff quiet %v %v", at, ok)
	}
}

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		level, color string
		want         severity.Level
		state        State
	}{
		{"ADVISORY", "YELLOW", severity.Moderate, Elevated},
		{"WATCH", "ORANGE", severity.Severe, Elevated},
		{"WARNING", "RED", severity.Extreme, Elevated},
		{"WATCH", "RED", severity.Severe, Elevated},         // ground decides
		{" watch ", "orange", severity.Severe, Elevated},    // case and space
		{"UNASSIGNED", "ORANGE", severity.Severe, Elevated}, // no ground level: aviation
		{"NORMAL", "YELLOW", 0, Quiet},                      // ground normal
		{"NORMAL", "GREEN", 0, Quiet},
		{"UNASSIGNED", "UNASSIGNED", 0, Quiet},
		{"UNASSIGNED", "GREEN", 0, Quiet},
		{"", "", 0, Unknown},            // null levels
		{"ELEVATED", "RED", 0, Unknown}, // unknown ground level
		{"UNASSIGNED", "PURPLE", 0, Unknown},
	} {
		got, state := Classify(c.level, c.color)
		if got != c.want || state != c.state {
			t.Errorf("%q/%q = %v %v, want %v %v", c.level, c.color, got, state, c.want, c.state)
		}
	}
	if s := sourceSeverity("UNASSIGNED", "ORANGE"); s != "aviation Orange" {
		t.Errorf("seamount source severity %q", s)
	}
}

// Null levels are unreadable, never quiet; an id twice, once elevated,
// stays elevated; history applies only where its levels match.
func TestParseEdgeCases(t *testing.T) {
	status := `{"features": [
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"volcanoCd": "a", "alertLevel": null, "colorCode": null}},
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"volcanoCd": "", "alertLevel": "WATCH", "alertDate": "2026-11-26 10:00:00"}},
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"volcanoCd": "d", "alertLevel": "WATCH", "colorCode": "ORANGE", "alertDate": "2026-11-26 10:00:00"}},
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"volcanoCd": "d", "alertLevel": "NORMAL", "colorCode": "GREEN"}},
	  {"geometry": {"type": "Point", "coordinates": [1, 2]}, "properties": {"volcanoCd": "m", "alertLevel": "WATCH", "colorCode": "ORANGE", "alertDate": "2026-11-26 10:00:00"}}
	]}`
	elevated := `[{"volcanoCd": "m", "alertLevel": "ADVISORY", "colorCode": "YELLOW", "codeChangeDate": "2026-01-01 00:00:00", "alertLevelPrev": "NORMAL"}]`
	f, err := Parse([]byte(status), []byte(elevated))
	if err != nil {
		t.Fatal(err)
	}
	if f.Unreadable != 2 || len(f.Events) != 2 || len(f.Quiet) != 0 || !f.Listed["a"] {
		t.Fatalf("unreadable %d, events %d, quiet %v, listed %v", f.Unreadable, len(f.Events), f.Quiet, f.Listed)
	}
	if m := byID(f.Events)["m"]; m.StartedAt != nil || m.Attrs["alertLevelPrev"] != nil {
		t.Fatalf("history for another level applied: %+v", m)
	}
	if _, err := Parse([]byte(`{"features": []}`), nil); err == nil {
		t.Error("an empty status list must fail the poll")
	}
	if _, err := Parse([]byte(status), []byte(`{`)); err == nil {
		t.Error("an unreadable elevated list must fail the poll")
	}
}

type fakeStore struct {
	got      []domain.Event
	starts   map[string]time.Time
	resolved map[string]time.Time
	order    []string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.order = append(f.order, "upsert")
	f.got = events
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ActiveStarts(context.Context, string) (map[string]time.Time, error) {
	return f.starts, nil
}

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
	s.now = func() time.Time { return now }
	return s
}

// A stored volcano back at normal ends at its notice when that is recent
// and after its start, else at the poll; one still elevated or no longer
// listed stays.
func TestPoll(t *testing.T) {
	status := strings.Replace(string(read(t, "geojson.json")), `"volcanoName":"Adagdak","vnum":"311800","volcanoCd":"ak3"`,
		`"volcanoName":"Adagdak","vnum":"311800","volcanoCd":"ak3","alertDate":"2026-11-25 16:00:00"`, 1)
	if !strings.Contains(status, `"alertDate":"2026-11-25 16:00:00"`) {
		t.Fatal("fixture edit missed")
	}
	st := &fakeStore{starts: map[string]time.Time{
		"hi3":  {},                                           // still elevated
		"ak3":  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), // quiet, recent notice after its start
		"wa4":  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),  // quiet, notice of 2026-10-15: too old
		"gone": {},                                           // not listed: kept
	}}
	res, err := newSource(t, []byte(status), read(t, "elevated.json"), st).Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.got) != 3 || st.order[0] != "upsert" {
		t.Fatalf("upserted %d, order %v", len(st.got), st.order)
	}
	if !st.resolved["ak3"].Equal(time.Date(2026, 11, 25, 16, 0, 0, 0, time.UTC)) || !st.resolved["wa4"].Equal(now) {
		t.Fatalf("resolved %v", st.resolved)
	}
	if _, ok := st.resolved["hi3"]; ok {
		t.Fatal("an elevated volcano ended")
	}
	if _, ok := st.resolved["gone"]; ok {
		t.Fatal("an unlisted volcano ended")
	}
	if res.Upserted != 3+len(st.resolved) {
		t.Fatalf("result %d", res.Upserted)
	}
}

// A failed history feed degrades: levels still stored, stored starts kept.
func TestPollWithoutHistory(t *testing.T) {
	kept := time.Date(2026, 7, 16, 0, 13, 50, 0, time.UTC)
	st := &fakeStore{starts: map[string]time.Time{"nmi2": kept}}
	if _, err := newSource(t, read(t, "geojson.json"), nil, st).Poll(t.Context(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	by := byID(st.got)
	if len(st.got) != 3 || by["nmi2"].StartedAt == nil || !by["nmi2"].StartedAt.Equal(kept) || by["hi3"].StartedAt != nil {
		t.Fatalf("%d stored, Ahyi start %v, Kilauea start %v", len(st.got), by["nmi2"].StartedAt, by["hi3"].StartedAt)
	}
}

// A failed status list writes nothing.
func TestPollFails(t *testing.T) {
	st := &fakeStore{starts: map[string]time.Time{"ak3": {}}}
	if _, err := newSource(t, nil, read(t, "elevated.json"), st).Poll(t.Context(), domain.Validators{}); err == nil || st.got != nil || st.resolved != nil {
		t.Fatalf("err %v, wrote %d, resolved %v", err, len(st.got), st.resolved)
	}
	if info := New(nil, nil, nil).Info(); info.ID != ID || !info.OfficialWarnings || len(info.Coverage) != 3 {
		t.Fatalf("%+v", info)
	}
}

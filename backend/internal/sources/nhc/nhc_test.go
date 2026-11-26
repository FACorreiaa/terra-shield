package nhc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type multiPolygon struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

func cone(t *testing.T, raw json.RawMessage) multiPolygon {
	t.Helper()
	var m multiPolygon
	if err := json.Unmarshal(raw, &m); err != nil || m.Type != "MultiPolygon" || len(m.Coordinates) == 0 {
		t.Fatalf("cone %s: %v", raw, err)
	}
	return m
}

// Gert, advisory 11 (NHC's JSON sample): the cone is one polygon, and the
// first point is the synoptic time, three hours before issuance.
func TestParseForecast(t *testing.T) {
	issuedAt := time.Date(2023, 8, 22, 3, 0, 0, 0, time.UTC)
	f, err := parseForecast(readFixture(t, "al062023_5day_011.zip"), issuedAt)
	if err != nil {
		t.Fatal(err)
	}
	c := cone(t, f.Cone)
	for _, poly := range c.Coordinates {
		for _, p := range poly[0] {
			if p[0] < -180 || p[0] > 180 {
				t.Fatalf("cone point %v out of range", p)
			}
		}
	}
	first := f.Points[0]
	if first.Tau != 0 || first.WindKt != 25 || !first.At.Equal(time.Date(2023, 8, 22, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("first point %+v", first)
	}
	for i, p := range f.Points {
		if p.At.IsZero() || (i > 0 && !p.At.Equal(first.At.Add(time.Duration(p.Tau)*time.Hour))) {
			t.Fatalf("point %d at %v, tau %d", i, p.At, p.Tau)
		}
		if p.Pressure >= missing || p.WindKt >= missing {
			t.Fatalf("point %d kept a missing value: %+v", i, p)
		}
	}
}

// Dora, advisory 40 (2023-08-10): NHC writes the cone from 159.9° to
// 190.7° and the track from -196° to -169.8°. Both come out within
// [-180, 180], the cone cut in two at the antimeridian.
func TestParseForecastAntimeridian(t *testing.T) {
	f, err := parseForecast(readFixture(t, "ep052023_5day_040.zip"), time.Date(2023, 8, 10, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	c := cone(t, f.Cone)
	if len(c.Coordinates) != 2 {
		t.Fatalf("cone has %d parts, want 2 (either side of the antimeridian)", len(c.Coordinates))
	}
	var east, west bool
	for _, poly := range c.Coordinates {
		for _, p := range poly[0] {
			if p[0] < -180 || p[0] > 180 {
				t.Fatalf("cone point %v out of range", p)
			}
			east = east || p[0] == 180
			west = west || p[0] == -180
		}
	}
	if !east || !west {
		t.Fatalf("parts do not meet at the antimeridian: east %v west %v", east, west)
	}
	for _, p := range f.Points {
		if p.Lon < -180 || p.Lon > 180 {
			t.Fatalf("track point %+v out of range", p)
		}
	}
	if last := f.Points[len(f.Points)-1]; last.Lon <= 0 {
		t.Fatalf("last point %v should be past the antimeridian, east longitude", last.Lon)
	}
}

func TestValidTime(t *testing.T) {
	at := func(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		name, s string
		approx  time.Time
		want    time.Time
	}{
		{"same month", "22/0000", at(2023, 8, 22, 3), at(2023, 8, 22, 0)},
		{"into next month", "01/0600", at(2023, 8, 31, 21), at(2023, 9, 1, 6)},
		{"from last month", "31/1800", at(2023, 9, 1, 3), at(2023, 8, 31, 18)},
		{"new year", "01/0000", at(2026, 12, 31, 21), at(2027, 1, 1, 0)},
		{"old year", "31/1800", at(2027, 1, 1, 3), at(2026, 12, 31, 18)},
		{"no such day", "31/0000", at(2023, 6, 30, 21), time.Time{}},
		{"too far", "10/0000", at(2023, 8, 22, 3), time.Time{}},
		{"unreadable", "0600", at(2023, 8, 22, 3), time.Time{}},
		{"blank", "", at(2023, 8, 22, 3), time.Time{}},
	} {
		if got := validTime(c.s, c.approx); !got.Equal(c.want) {
			t.Errorf("%s: validTime(%q, %v) = %v, want %v", c.name, c.s, c.approx, got, c.want)
		}
	}
}

func TestSplitAntimeridian(t *testing.T) {
	// 170°E to 190°E (170°W), clockwise.
	ring := [][2]float64{{170, 10}, {170, 20}, {190, 20}, {190, 10}, {170, 10}}
	parts := splitAntimeridian([][][2]float64{ring})
	if len(parts) != 2 {
		t.Fatalf("%d parts", len(parts))
	}
	for _, p := range parts {
		if a := signedArea(p[0]); a > -99.9 || a < -100.1 {
			t.Fatalf("part %v area %v, want 10° x 10° clockwise", p[0], a)
		}
	}
	within := [][2]float64{{10, 10}, {10, 20}, {20, 20}, {20, 10}, {10, 10}}
	if parts := splitAntimeridian([][][2]float64{within}); len(parts) != 1 || !slices.Equal(parts[0][0], within) {
		t.Fatalf("a polygon within range changed: %v", parts)
	}
}

func TestParseCurrent(t *testing.T) {
	if s, bad, err := parseCurrent([]byte(`{"activeStorms": []}`)); err != nil || len(s) != 0 || len(bad) != 0 {
		t.Fatalf("empty list: %v %v %v", s, bad, err)
	}
	for name, body := range map[string]string{
		"no list":  `{}`,
		"null":     `{"activeStorms": null}`,
		"not json": `<html>`,
	} {
		if _, _, err := parseCurrent([]byte(body)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	s, bad, err := parseCurrent([]byte(`{"activeStorms": [{"id": "xx012023"}, {"id": "AL012023"}]}`))
	if err != nil || len(s) != 1 || s[0].ID != "al012023" || !slices.Equal(bad, []string{"xx012023"}) {
		t.Fatalf("bad id: %v %v %v", s, bad, err)
	}
	storms, _, err := parseCurrent(readFixture(t, "current.json"))
	if err != nil || len(storms) != 4 {
		t.Fatalf("sample: %d storms, %v", len(storms), err)
	}
}

func TestSeverity(t *testing.T) {
	for _, c := range []struct {
		kt   float64
		want string
	}{{0, "minor"}, {63, "minor"}, {64, "moderate"}, {95, "moderate"}, {96, "severe"}, {136, "severe"}, {137, "extreme"}} {
		ev := buildWith(t, c.kt, c.kt)
		if ev.Severity != c.want {
			t.Errorf("%v kt: %s, want %s", c.kt, ev.Severity, c.want)
		}
	}
	for _, c := range []struct {
		now, peak float64
		want      string
	}{
		{100, 100, "Cat 3"},
		{100, 115, "Cat 3 now, Cat 4 forecast"},
		{100, 110, "Cat 3"},
		{45, 60, "45 kt now, 60 kt forecast"},
		{115, 90, "Cat 4"},
		{0, 50, "50 kt forecast"},
		{0, 0, ""},
	} {
		if got := sourceSeverity(c.now, c.peak); got != c.want {
			t.Errorf("sourceSeverity(%v, %v) = %q, want %q", c.now, c.peak, got, c.want)
		}
	}
	// The forecast peak rates the storm: a tropical storm forecast to be a
	// major hurricane is severe now.
	if ev := buildWith(t, 50, 100); ev.Severity != "severe" {
		t.Fatalf("peak not counted: %s", ev.Severity)
	}
}

func buildWith(t *testing.T, now, peak float64) domain.Event {
	t.Helper()
	intensity := ""
	if now > 0 {
		intensity = strconv.FormatFloat(now, 'f', -1, 64)
	}
	st := &storm{
		ID: "al012026", Name: "Test", Classification: "TS", Intensity: intensity, LastUpdate: "2026-08-01T03:00:00.000Z",
		TrackCone: &product{AdvNum: "001", Issuance: "2026-08-01T03:00:00.000Z"},
	}
	f := &forecast{Cone: json.RawMessage(`{"type":"MultiPolygon","coordinates":[]}`), Points: []point{
		{Tau: 0, Lon: -50, Lat: 15, WindKt: now}, {Tau: 72, Lon: -60, Lat: 20, WindKt: peak},
	}}
	ev, err := build(st, f)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// An intermediate advisory moves the storm without a new forecast: the
// pin, figures and advisory number are the public advisory's, the cone
// and track the forecast's.
func TestBuildIntermediateAdvisory(t *testing.T) {
	lat, lon, dir, mph := 17.4, -59.0, 285.0, 9.0
	st := &storm{
		ID: "al062023", Name: "Gert", Classification: "TD", Intensity: "30", Pressure: "1007",
		LatitudeNumeric: &lat, LongitudeNumeric: &lon, MovementDir: &dir, MovementSpeed: &mph,
		LastUpdate:       "2023-08-22T06:00:00.000Z",
		PublicAdvisory:   &product{AdvNum: "011A", Issuance: "2023-08-22T06:00:00.000Z", URL: "https://www.nhc.noaa.gov/text/MIATCPAT1.shtml"},
		ForecastGraphics: &product{AdvNum: "011", URL: "https://www.nhc.noaa.gov/graphics_at1.shtml"},
		TrackCone:        &product{AdvNum: "011", Issuance: "2023-08-22T03:00:00.000Z"},
	}
	f, err := parseForecast(readFixture(t, "al062023_5day_011.zip"), time.Date(2023, 8, 22, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	ev, err := build(st, f)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Position == nil || *ev.Position != [2]float64{-59.0, 17.4} {
		t.Fatalf("pin %v, want the public advisory's center", ev.Position)
	}
	if ev.Attrs["advisory"] != "011A" || ev.Attrs["forecastAdvisory"] != "011" || ev.Attrs["windKt"] != 30.0 || ev.Attrs["pressureMb"] != 1007.0 {
		t.Fatalf("attrs %v", ev.Attrs)
	}
	if !ev.UpdatedAt.Equal(time.Date(2023, 8, 22, 6, 0, 0, 0, time.UTC)) || ev.Title != "Tropical Depression Gert" ||
		ev.URL != "https://www.nhc.noaa.gov/graphics_at1.shtml" || ev.Kind != domain.KindForecast {
		t.Fatalf("event %+v", ev)
	}
	if string(ev.Geometry) != string(f.Cone) {
		t.Fatal("geometry is not the forecast's cone")
	}
}

// fakeStore records writes; stored is what it holds active.
type fakeStore struct {
	mu       sync.Mutex
	stored   []string
	upserted [][]domain.Event
	resolved [][]string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserted = append(f.upserted, events)
	return len(events), nil
}

func (f *fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ActiveIDs(context.Context, string) ([]string, error) {
	return slices.Clone(f.stored), nil
}

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved = append(f.resolved, ids)
	return len(ids), nil
}

// stub serves the sample list with its archive links pointed at itself:
// Gert's and Franklin's archives exist, Nine's is not yet published
// (404), Hilary has no cone.
func stub(t *testing.T) (*httptest.Server, *sync.Map, func(string)) {
	t.Helper()
	srv, hits, set, _ := stubWith(t)
	return srv, hits, set
}

// stubWith also returns alias: serve a fixture under another archive
// name (an archive published later, or a status code as "429").
func stubWith(t *testing.T) (*httptest.Server, *sync.Map, func(string), func(name, as string)) {
	t.Helper()
	var mu sync.Mutex
	aliases := map[string]string{}
	hits := &sync.Map{}
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := hits.LoadOrStore(r.URL.Path, new(int))
		mu.Lock()
		*n.(*int)++
		b := body
		mu.Unlock()
		if r.URL.Path == "/CurrentStorms.json" {
			_, _ = w.Write([]byte(b))
			return
		}
		name := path.Base(r.URL.Path)
		mu.Lock()
		as, ok := aliases[name]
		mu.Unlock()
		if ok {
			if as == "429" {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			name = as
		}
		zip, err := os.ReadFile("testdata/" + name)
		if err != nil || !strings.HasSuffix(name, ".zip") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(zip)
	}))
	t.Cleanup(srv.Close)
	set := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		body = strings.ReplaceAll(s, "https://www.nhc.noaa.gov/gis/forecast/archive/", srv.URL+"/archive/")
	}
	set(string(readFixture(t, "current.json")))
	alias := func(name, as string) {
		mu.Lock()
		defer mu.Unlock()
		aliases[name] = as
	}
	return srv, hits, set, alias
}

func newSource(t *testing.T, srv *httptest.Server, st Store) *Source {
	t.Helper()
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), st, slog.New(slog.DiscardHandler))
	s.url = srv.URL + "/CurrentStorms.json"
	u, _ := url.Parse(srv.URL)
	s.allowed = func(l *url.URL) bool { return l.Host == u.Host }
	return s
}

func TestPoll(t *testing.T) {
	srv, hits, set := stub(t)
	st := &fakeStore{stored: []string{
		"al092023", // Nine: listed, archive missing: kept
		"ep092023", // Hilary: listed, no cone: kept
		"al012023", // no longer listed: ended
	}}
	s := newSource(t, srv, st)
	ctx := context.Background()

	res, err := s.Poll(ctx, domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range st.upserted[0] {
		ids = append(ids, e.SourceEventID)
	}
	if !slices.Equal(ids, []string{"al062023", "al082023"}) {
		t.Fatalf("stored %v", ids)
	}
	if len(st.resolved) != 1 || !slices.Equal(st.resolved[0], []string{"al012023"}) {
		t.Fatalf("resolved %v", st.resolved)
	}
	if res.Validators != (domain.Validators{}) {
		t.Fatalf("validators kept while an archive is missing: %+v", res.Validators)
	}

	// The next poll reads each published archive from memory, and asks
	// again for the missing one.
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	count := func(p string) int {
		n, ok := hits.Load(p)
		if !ok {
			return 0
		}
		return *n.(*int)
	}
	if g, n := count("/archive/al062023_5day_011.zip"), count("/archive/al092023_5day_003.zip"); g != 1 || n != 2 {
		t.Fatalf("Gert's archive fetched %d times, Nine's %d", g, n)
	}

	// A list without its storms key is an error, and nothing ends.
	set(`{"storms": []}`)
	before := len(st.resolved)
	if _, err := s.Poll(ctx, domain.Validators{}); err == nil || len(st.resolved) != before {
		t.Fatalf("malformed list: err %v, resolved %v", err, st.resolved)
	}
	// An empty list ends every storm.
	set(`{"activeStorms": []}`)
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if last := st.resolved[len(st.resolved)-1]; len(last) != 3 {
		t.Fatalf("empty list ended %v", last)
	}
}

// Archive links off NHC's host are not followed; the storm is kept.
func TestPollForeignArchive(t *testing.T) {
	srv, hits, _ := stub(t)
	st := &fakeStore{stored: []string{"al062023"}}
	s := newSource(t, srv, st)
	s.allowed = New(nil, nil, nil).allowed // the production rule: https://www.nhc.noaa.gov only
	if _, err := s.Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if len(st.upserted[0]) != 0 || len(st.resolved[0]) != 0 {
		t.Fatalf("upserted %v, resolved %v", st.upserted, st.resolved)
	}
	if _, ok := hits.Load("/archive/al062023_5day_011.zip"); ok {
		t.Fatal("followed a link off NHC's host")
	}
}

// NHC's cones are wound counter-clockwise, against the shapefile spec: a
// second ring wound the same way is a second part, one wound the other
// way a hole.
func TestConeRings(t *testing.T) {
	ccw := func(x, y, w float64) [][2]float64 {
		return [][2]float64{{x, y}, {x + w, y}, {x + w, y + w}, {x, y + w}, {x, y}}
	}
	cw := func(x, y, w float64) [][2]float64 { r := ccw(x, y, w); slices.Reverse(r); return r }
	for name, c := range map[string]struct {
		records [][][][2]float64
		parts   []int // rings per part
	}{
		"two parts, one record":     {[][][][2]float64{{ccw(0, 0, 10), ccw(20, 0, 10)}}, []int{1, 1}},
		"two records":               {[][][][2]float64{{ccw(0, 0, 10)}, {ccw(20, 0, 10)}}, []int{1, 1}},
		"hole":                      {[][][][2]float64{{ccw(0, 0, 10), cw(2, 2, 2)}}, []int{2}},
		"clockwise, as the spec":    {[][][][2]float64{{cw(0, 0, 10), ccw(2, 2, 2), cw(20, 0, 10)}}, []int{2, 1}},
		"crossing the antimeridian": {[][][][2]float64{{ccw(175, 0, 10)}}, []int{1, 1}},
	} {
		raw, err := coneGeoJSON(c.records)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		m := cone(t, raw)
		var got []int
		for _, p := range m.Coordinates {
			got = append(got, len(p))
		}
		if !slices.Equal(got, c.parts) {
			t.Errorf("%s: rings per part %v, want %v", name, got, c.parts)
		}
	}
}

// An archive missing on one poll is read when published; a new advisory's
// archive replaces the old one in memory; a republished archive (new file
// time) is read again.
func TestPollArchiveLifecycle(t *testing.T) {
	srv, hits, set, alias := stubWith(t)
	st := &fakeStore{}
	s := newSource(t, srv, st)
	ctx := context.Background()
	count := func(p string) int {
		n, ok := hits.Load(p)
		if !ok {
			return 0
		}
		return *n.(*int)
	}
	stored := func() []string {
		var ids []string
		for _, e := range st.upserted[len(st.upserted)-1] {
			ids = append(ids, e.SourceEventID)
		}
		return ids
	}
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if ids := stored(); slices.Contains(ids, "al092023") {
		t.Fatalf("Nine stored before its archive: %v", ids)
	}
	alias("al092023_5day_003.zip", "al062023_5day_011.zip") // published now
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if ids := stored(); !slices.Contains(ids, "al092023") {
		t.Fatalf("Nine not stored once published: %v", ids)
	}
	if len(s.archives) != 3 {
		t.Fatalf("%d archives in memory, want 3", len(s.archives))
	}

	body := string(readFixture(t, "current.json"))
	next := strings.ReplaceAll(body, "al062023_5day_011.zip", "al062023_5day_012.zip")
	alias("al062023_5day_012.zip", "al062023_5day_011.zip")
	set(next)
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if count("/archive/al062023_5day_012.zip") != 1 || len(s.archives) != 3 {
		t.Fatalf("new advisory: fetched %d, %d in memory", count("/archive/al062023_5day_012.zip"), len(s.archives))
	}
	for key := range s.archives {
		if strings.Contains(key, "al062023_5day_011.zip") {
			t.Fatal("the replaced advisory's archive is still in memory")
		}
	}
	// Every product republished with a new file time.
	set(strings.ReplaceAll(next, `"fileUpdateTime": "2023-08-22T02:55:52.439Z"`, `"fileUpdateTime": "2023-08-22T03:10:00.000Z"`))
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if n := count("/archive/al062023_5day_012.zip"); n != 2 {
		t.Fatalf("republished archive fetched %d times, want 2", n)
	}
}

// A 429 on an archive fails the poll, so it backs off rather than asking
// again in two minutes; nothing is ended.
func TestPollArchiveRateLimited(t *testing.T) {
	srv, _, _, alias := stubWith(t)
	alias("al062023_5day_011.zip", "429")
	st := &fakeStore{stored: []string{"al012023"}}
	s := newSource(t, srv, st)
	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil {
		t.Fatal("no error on 429")
	}
	if len(st.resolved) != 0 || len(st.upserted) != 0 {
		t.Fatalf("wrote on 429: %v %v", st.upserted, st.resolved)
	}
}

// One unreadable id: the rest still update, and nothing ends.
func TestPollUnreadableID(t *testing.T) {
	srv, _, set := stub(t)
	set(strings.Replace(string(readFixture(t, "current.json")), `"id": "al082023"`, `"id": "al08-2023"`, 1))
	st := &fakeStore{stored: []string{"al082023", "al012023"}}
	s := newSource(t, srv, st)
	if _, err := s.Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if len(st.upserted) != 1 || len(st.upserted[0]) != 1 || st.upserted[0][0].SourceEventID != "al062023" {
		t.Fatalf("upserted %v", st.upserted)
	}
	if len(st.resolved) != 0 {
		t.Fatalf("ended %v while an id was unreadable", st.resolved)
	}
}

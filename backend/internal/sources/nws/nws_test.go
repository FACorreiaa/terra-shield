package nws

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

// The fixture is nine real alerts (2026-11-26; public domain):
//   - KPQR.CF.Y.0005, one coastal flood advisory in two segments (OR, WA);
//   - a Small Craft Advisory (marine zones only);
//   - KILX.WW.Y.0003, canceled (CAN);
//   - a Special Weather Statement (no VTEC);
//   - KJAX.CF.W.0004, extended (EXT);
//   - KMHX.FF.W.0008, a storm-based Flash Flood Warning (a polygon, counties);
//   - KMFL.TO.A.0706, a Tornado Watch by county, expired (EXP);
//   - KCYS.FW.W.0048, a Red Flag Warning by fire zone, expired (EXP).
func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/active.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func byKey(p Parsed) map[string]Warning {
	out := map[string]Warning{}
	for i := range p.Warnings {
		out[p.Warnings[i].Key] = p.Warnings[i]
	}
	return out
}

func TestParse(t *testing.T) {
	p, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if p.Listed != 9 || p.Unreadable != 0 || p.Skipped[SkipMarine] != 1 || p.Skipped[SkipNoVTEC] != 1 {
		t.Fatalf("listed %d unreadable %d skipped %v", p.Listed, p.Unreadable, p.Skipped)
	}
	ws := byKey(p)
	if len(ws) != 3 {
		t.Fatalf("warnings %v", slices.Collect(maps.Keys(ws)))
	}
	// Ended events are not live: a cancellation and two expiries.
	for _, k := range []string{"KILX.WW.Y.0003", "KMFL.TO.A.0706", "KCYS.FW.W.0048"} {
		if p.Live[k] {
			t.Errorf("%s is over but live", k)
		}
	}
	// The marine advisory is live (a skip never ends an event) but not stored.
	if !p.Live["KMOB.SC.Y.0038"] {
		t.Error("a skipped alert's event must still count as listed")
	}

	cf := ws["KPQR.CF.Y.0005"]
	if len(cf.Zones) != 2 || cf.Zones[0] != (domain.ZoneRef{Set: "forecast", UGC: "ORZ101"}) || cf.Zones[1].UGC != "WAZ201" || len(cf.Polygons) != 0 {
		t.Fatalf("two segments, one area: %+v", cf.Zones)
	}
	e := cf.Event
	if e.Type != domain.TypeFlood || e.Kind != domain.KindWarning || e.Severity != "minor" || e.SourceSeverity != "Minor (advisory)" ||
		!strings.HasPrefix(e.Title, "Coastal Flood Advisory: ") || e.URL != "https://www.weather.gov/pqr/" ||
		!slices.Equal(e.CountryRule.Listed, []string{"US"}) || e.Attrs["office"] != "KPQR" || e.Summary == "" {
		t.Fatalf("event %+v", e)
	}
	if _, ok := e.Attrs["endsAt"].(time.Time); !ok {
		t.Errorf("endsAt %v", e.Attrs["endsAt"])
	}

	ff := ws["KMHX.FF.W.0008"]
	if len(ff.Polygons) != 1 || len(ff.Zones) != 0 || ff.Event.Severity != "severe" {
		t.Fatalf("a storm-based warning is its polygon, not its counties: %+v", ff)
	}
}

// The expired watch and Red Flag Warning, put back in force: counties and
// fire zones are named as such.
func TestParseZoneSets(t *testing.T) {
	body := strings.NewReplacer("/O.EXP.KMFL.TO.A.0706.", "/O.CON.KMFL.TO.A.0706.", "/O.EXP.KCYS.FW.W.0048.", "/O.CON.KCYS.FW.W.0048.").Replace(string(fixture(t)))
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	ws := byKey(p)
	to, fw := ws["KMFL.TO.A.0706"], ws["KCYS.FW.W.0048"]
	if len(to.Zones) == 0 || to.Zones[0].Set != "county" || to.Event.Type != domain.TypeStorm || to.Event.Severity != "extreme" {
		t.Fatalf("tornado watch %+v %+v", to.Zones, to.Event)
	}
	if len(fw.Zones) == 0 || fw.Zones[0].Set != "fire" || fw.Event.Type != domain.TypeWildfire {
		t.Fatalf("red flag %+v %+v", fw.Zones, fw.Event)
	}
	if !p.Live["KMFL.TO.A.0706"] || !p.Live["KCYS.FW.W.0048"] {
		t.Fatal("in force again, so live")
	}
}

func TestParseRejects(t *testing.T) {
	for name, body := range map[string]string{
		"not json":   "nope",
		"no list":    `{"type":"FeatureCollection"}`,
		"empty list": `{"type":"FeatureCollection","features":[]}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	// A tsunami product is the tsunami source's; an unreadable VTEC is
	// counted, and blocks ending events.
	body := strings.Replace(string(fixture(t)), "/O.EXT.KJAX.CF.W.0004.", "/O.EXT.KJAX.TS.W.0004.", 1)
	body = strings.Replace(body, "/O.NEW.KMHX.FF.W.0008.", "/O.NEW.KMHX.FF.W.08.", 1)
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	ws := byKey(p)
	if _, ok := ws["KJAX.TS.W.0004"]; ok || p.Skipped[SkipTsunami] != 1 || p.Unreadable != 1 {
		t.Fatalf("tsunami %v unreadable %d", p.Skipped, p.Unreadable)
	}
}

type fakeStore struct {
	mu       sync.Mutex
	active   []string
	upserted []domain.Event
	resolved []string
}

func (f *fakeStore) Upsert(_ context.Context, _ string, es []domain.Event) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserted = es
	return len(es), nil
}

func (f *fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ActiveIDs(context.Context, string) ([]string, error) { return f.active, nil }

func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	f.resolved = append(f.resolved, ids...)
	return len(ids), nil
}

// drawer draws any zone but XXZ999 as a unit square.
type drawer struct{}

func (drawer) Draw(_ context.Context, zones []domain.ZoneRef, polygons []json.RawMessage) (json.RawMessage, []string, error) {
	var found []string
	for _, z := range zones {
		if z.UGC != "XXZ999" {
			found = append(found, z.Set+"/"+z.UGC)
		}
	}
	if len(found) == 0 && len(polygons) == 0 {
		return nil, nil, nil
	}
	return json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[0,0],[1,0],[1,1],[0,1],[0,0]]]]}`), found, nil
}

func newTestSource(t *testing.T, body string, store Store) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, drawer{}, slog.New(slog.DiscardHandler))
	s.url = srv.URL
	s.now = func() time.Time { return time.Date(2027, 1, 1, 0, 30, 0, 0, time.UTC) }
	return s
}

func TestPoll(t *testing.T) {
	// Over New Year: KPQR.CF.Y.0005 was first seen in 2026 and keeps its
	// id; KJAX.CF.W.0004 is new and takes 2027; a stored event the list no
	// longer holds in force (canceled, or gone) ends.
	store := &fakeStore{active: []string{"KPQR.CF.Y.0005.2026", "KILX.WW.Y.0003.2026", "KBOX.HW.W.0001.2026"}}
	s := newTestSource(t, string(fixture(t)), store)
	res, err := s.Poll(context.Background(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range store.upserted {
		ids[e.SourceEventID] = true
		if e.Geometry == nil {
			t.Errorf("%s undrawn", e.SourceEventID)
		}
	}
	if !ids["KPQR.CF.Y.0005.2026"] || !ids["KJAX.CF.W.0004.2027"] || !ids["KMHX.FF.W.0008.2027"] || len(ids) != 3 {
		t.Fatalf("ids %v", ids)
	}
	slices.Sort(store.resolved)
	if !slices.Equal(store.resolved, []string{"KBOX.HW.W.0001.2026", "KILX.WW.Y.0003.2026"}) {
		t.Fatalf("resolved %v", store.resolved)
	}
	if res.Validators.ETag != `"v1"` || res.Upserted != 5 {
		t.Fatalf("result %+v", res)
	}
	// Unchanged: the server answers 304 and nothing is written.
	if res, err := s.Poll(context.Background(), res.Validators); err != nil || !res.NotModified {
		t.Fatalf("not modified %+v %v", res, err)
	}
}

func TestPollKeepsEventsWhenUnsure(t *testing.T) {
	// An unreadable VTEC could be about any stored event: nothing ends.
	body := strings.Replace(string(fixture(t)), "/O.NEW.KMHX.FF.W.0008.", "/O.NEW.KMHX.FF.W.08.", 1)
	store := &fakeStore{active: []string{"KMHX.FF.W.0008.2026", "KBOX.HW.W.0001.2026"}}
	if _, err := newTestSource(t, body, store).Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if len(store.resolved) != 0 {
		t.Fatalf("ended %v while an alert was unreadable", store.resolved)
	}
	// An event whose zones cannot be drawn stays listed: kept, not ended.
	body = strings.ReplaceAll(string(fixture(t)), `"ORZ101"`, `"XXZ999"`)
	body = strings.ReplaceAll(body, `"WAZ201"`, `"XXZ999"`)
	store = &fakeStore{active: []string{"KPQR.CF.Y.0005.2026"}}
	if _, err := newTestSource(t, body, store).Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	for _, e := range store.upserted {
		if e.SourceEventID == "KPQR.CF.Y.0005.2026" {
			t.Fatal("undrawn event upserted")
		}
	}
	if len(store.resolved) != 0 {
		t.Fatalf("undrawn but listed event ended: %v", store.resolved)
	}
	// An empty list is an outage, not the end of every warning.
	store = &fakeStore{active: []string{"KPQR.CF.Y.0005.2026"}}
	if _, err := newTestSource(t, `{"type":"FeatureCollection","features":[]}`, store).Poll(context.Background(), domain.Validators{}); err == nil || len(store.resolved) != 0 {
		t.Fatalf("empty list: err %v resolved %v", err, store.resolved)
	}
}

func TestTitleAndOffice(t *testing.T) {
	if got := title("Wind Advisory", []string{"A", "B", "C", "D", "E"}); got != "Wind Advisory: A, B, C and 2 more" {
		t.Error(got)
	}
	for office, want := range map[string]string{
		"KPAH": "https://www.weather.gov/pah/", "PAFC": "https://www.weather.gov/afc/",
		"TJSJ": "https://www.weather.gov/sju/", "KWNS": "https://www.spc.noaa.gov/",
	} {
		if got := officeURL(office); got != want {
			t.Errorf("%s: %s", office, got)
		}
	}
}

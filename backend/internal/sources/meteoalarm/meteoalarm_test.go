package meteoalarm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
)

// recordedAt is when testdata/feeds-germany.json was fetched; its warnings
// date from 2026-11-10 to 2026-11-20, so some have expired.
var recordedAt = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestParseRecordedFeed runs over 8 DWD warnings cut from the real German
// feed (English and German blocks kept): one per awareness type, plus the
// red and orange ones.
func TestParseRecordedFeed(t *testing.T) {
	warnings, _, err := Parse(read(t, "feeds-germany.json"), recordedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 8 {
		t.Fatalf("%d warnings, want 8", len(warnings))
	}
	byType := map[domain.EventType]int{}
	for i := range warnings {
		w := &warnings[i]
		byType[w.Type]++
		if w.Kind != domain.KindWarning || w.Title == "" || len(w.Areas) == 0 || w.Geometry != nil ||
			w.Attrs["language"] != "en" || w.URL != "https://dwd.de/warnungen" {
			t.Fatalf("incomplete %+v", w.Event)
		}
		if (w.Status == domain.StatusResolved) != (w.EndedAt != nil && !w.EndedAt.After(recordedAt)) {
			t.Fatalf("%s status %s ended %v", w.SourceEventID, w.Status, w.EndedAt)
		}
		if sender, _ := w.Attrs["sender"].(string); !strings.Contains(sender, "Deutscher Wetterdienst") && !strings.HasPrefix(sender, "DWD") {
			t.Fatalf("sender %v", w.Attrs["sender"])
		}
	}
	for _, typ := range []domain.EventType{domain.TypeHeavyRain, domain.TypeStorm, domain.TypeSnowIce, domain.TypeCold, domain.TypeWind, domain.TypeOther} {
		if byType[typ] == 0 {
			t.Errorf("no %s warning in %v", typ, byType)
		}
	}

	red := warnings[0]
	if red.Type != domain.TypeHeavyRain || red.Severity != "extreme" || red.SourceSeverity != "Red warning" ||
		red.Title != "Official SEVERE WEATHER WARNING of HEAVY PERSISTENT RAIN" || len(red.Areas) != 12 {
		t.Errorf("red rain %+v areas %d", red.Event, len(red.Areas))
	}
	if warnings[1].Severity != "severe" || len(warnings[1].Supersedes) == 0 {
		t.Errorf("orange update %+v supersedes %v", warnings[1].Event, warnings[1].Supersedes)
	}
}

const edgeCases = `{"warnings": [
 {"alert": {"identifier": "cancel-1", "status": "Actual", "msgType": "Cancel", "sent": "2026-11-16T10:00:00Z",
   "references": "sender,old-1,2026-11-15T10:00:00Z sender,old-2,2026-11-15T11:00:00Z", "info": []}},
 {"alert": {"identifier": "green", "status": "Actual", "msgType": "Alert", "sent": "2026-11-16T10:00:00Z", "info": [
   {"language": "en", "expires": "2026-11-17T00:00:00Z", "parameter": [{"valueName": "awareness_level", "value": "1; green; Minor"}],
    "area": [{"areaDesc": "A", "geocode": [{"valueName": "EMMA_ID", "value": "PT001"}]}]}]}},
 {"alert": {"identifier": "exercise", "status": "Exercise", "msgType": "Alert", "sent": "2026-11-16T10:00:00Z", "info": []}},
 {"alert": {"identifier": "pt-heat", "status": "Actual", "msgType": "Alert", "sent": "2026-11-16T10:00:00Z", "info": [
   {"language": "pt-PT", "event": "Tempo quente", "headline": "", "expires": "2026-11-17T00:00:00Z", "onset": "not a time",
    "description": "Calor.", "instruction": "Beba água.",
    "parameter": [{"valueName": "awareness_level", "value": "3"}, {"valueName": "awareness_type", "value": "5; high-temperature"}],
    "area": [{"areaDesc": "Lisboa", "geocode": [{"valueName": "EMMA_ID", "value": "PT011"}]},
             {"areaDesc": "Lisboa again", "geocode": [{"valueName": "EMMA_ID", "value": "PT011"}]}]}]}},
 {"alert": {"identifier": "no-areas", "status": "Actual", "msgType": "Alert", "sent": "2026-11-16T10:00:00Z", "info": [
   {"language": "en", "expires": "2026-11-17T00:00:00Z", "parameter": [{"valueName": "awareness_level", "value": "2; yellow; Moderate"}], "area": []}]}},
 {"alert": {"identifier": "odd-type", "status": "Actual", "msgType": "Alert", "sent": "2026-11-16T10:00:00Z", "info": [
   {"language": "en", "event": "Something", "expires": "2026-11-17T00:00:00Z",
    "parameter": [{"valueName": "awareness_level", "value": "2; yellow; Moderate"}, {"valueName": "awareness_type", "value": "99; new"}],
    "area": [{"areaDesc": "B", "geocode": [{"valueName": "EMMA_ID", "value": "ES001"}]}]}]}}
]}`

func TestParseEdgeCases(t *testing.T) {
	warnings, skipped, err := Parse([]byte(edgeCases), recordedAt)
	if err != nil {
		t.Fatal(err)
	}
	// green is dropped by design and uncounted; the exercise is not a warning.
	if len(skipped) != 1 || skipped[SkipNoArea] != 1 {
		t.Errorf("skipped %v, want only no-areas as %s", skipped, SkipNoArea)
	}
	if len(warnings) != 3 {
		t.Fatalf("kept %d, want cancel-1, pt-heat, odd-type", len(warnings))
	}
	if c := warnings[0]; !c.Cancel || strings.Join(c.Supersedes, ",") != "old-1,old-2" {
		t.Errorf("cancel %+v", c)
	}
	heat := warnings[1]
	if heat.Type != domain.TypeHeat || heat.Severity != "severe" || heat.SourceSeverity != "Awareness level 3" ||
		heat.Title != "Tempo quente" || heat.Summary != "Calor.\n\nBeba água." || heat.URL != "https://meteoalarm.org/" ||
		len(heat.Areas) != 2 || fmt.Sprint(heat.Attrs["emmaIds"]) != "[PT011]" || !heat.StartedAt.Equal(time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("non-English heat %+v areas %v", heat.Event, heat.Areas)
	}
	if odd := warnings[2]; odd.Type != domain.TypeOther || odd.Status != domain.StatusActive {
		t.Errorf("unknown type %+v", odd.Event)
	}
}

func TestCAPAreas(t *testing.T) {
	emma := valuePair{"EMMA_ID", "FR026"}
	nuts := valuePair{"NUTS3", "FR712"}
	cases := []struct {
		name     string
		polygons []string
		geocodes []valuePair
		want     string // polygons as JSON, or codes as type:value
		ok       bool
	}{
		{
			"closed polygon wins over codes",
			[]string{"46.5,6.3 46.6,6.3 46.6,6.4 46.5,6.3"},
			[]valuePair{emma},
			`{"coordinates":[[[6.3,46.5],[6.3,46.6],[6.4,46.6],[6.3,46.5]]],"type":"Polygon"}`, true,
		},
		{
			"open ring closed",
			[]string{"46.5,6.3 46.6,6.3 46.6,6.4"},
			nil,
			`{"coordinates":[[[6.3,46.5],[6.3,46.6],[6.4,46.6],[6.3,46.5]]],"type":"Polygon"}`, true,
		},
		{"EMMA kept alone", nil, []valuePair{nuts, emma}, "EMMA_ID:FR026", true},
		{"alias when no EMMA", nil, []valuePair{nuts, {"NUTS3", " "}}, "NUTS3:FR712", true},
		{"bad polygon falls back to codes", []string{"46.5,6.3 nonsense"}, []valuePair{emma}, "EMMA_ID:FR026", true},
		{"out of range", []string{"95,6.3 46.6,6.3 46.6,6.4 95,6.3"}, nil, "", false},
		{"too few points", []string{"46.5,6.3 46.6,6.3 46.5,6.3"}, nil, "", false},
		{"nothing", nil, nil, "", false},
	}
	for _, c := range cases {
		a, ok := capArea(c.polygons, c.geocodes)
		var got []string
		for _, p := range a.Polygons {
			got = append(got, string(p))
		}
		for _, k := range a.Codes {
			got = append(got, k.Type+":"+k.Value)
		}
		if ok != c.ok || strings.Join(got, " ") != c.want {
			t.Errorf("%s: %v %q, want %v %q", c.name, ok, got, c.ok, c.want)
		}
	}
}

func TestReferences(t *testing.T) {
	got := references("a,id-1,2026 b,id-2,2026 malformed ,,x")
	if strings.Join(got, ",") != "id-1,id-2" {
		t.Fatalf("%v", got)
	}
}

// fakeAreas knows every code except "UNKNOWN".
type fakeAreas struct{}

func (fakeAreas) Resolve(_ context.Context, areas []Area) (json.RawMessage, bool, error) {
	for _, a := range areas {
		for _, c := range a.Codes {
			if c.Value == "UNKNOWN" {
				return nil, false, nil
			}
		}
	}
	return json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`), true, nil
}

type fakeStore struct {
	upserted []domain.Event
	calls    int // Upsert calls: one per country that answered
	deleted  []string
	order    []string // "upsert" and "delete", in call order
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.calls++
	f.order = append(f.order, "upsert")
	f.upserted = append(f.upserted, events...)
	return len(events), nil
}

func (f *fakeStore) Delete(_ context.Context, _ string, ids []string) (int, error) {
	f.order = append(f.order, "delete")
	f.deleted = append(f.deleted, ids...)
	return 0, nil
}

var _ ingest.EventStore = (*fakeStore)(nil)

func newTestSource(t *testing.T, handler http.HandlerFunc, countries ...Country) (*Source, *fakeStore) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	store := &fakeStore{}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, fakeAreas{}, slog.New(slog.DiscardHandler))
	s.base, s.countries = srv.URL+"/feeds-", countries
	s.now = func() time.Time { return recordedAt }
	return s, store
}

func TestPoll(t *testing.T) {
	germany := read(t, "feeds-germany.json")
	unknownArea := strings.Replace(edgeCases, `"PT011"`, `"UNKNOWN"`, 2)
	s, store := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feeds-germany":
			_, _ = w.Write(germany)
		case "/feeds-portugal":
			_, _ = w.Write([]byte(unknownArea))
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
	}, Country{"DE", "germany"}, Country{"PT", "portugal"}, Country{"FR", "france"})

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatalf("one failing country must not fail the poll: %v", err)
	}
	// 8 German + odd-type; the Portuguese heat warning has an unknown area.
	if len(store.upserted) != 9 || res.Upserted != 9 || store.calls != 2 {
		t.Fatalf("upserted %d in %d calls, result %+v", len(store.upserted), store.calls, res)
	}
	// Each country's superseded warnings go right after its own upsert.
	if got := strings.Join(store.order, ","); got != "upsert,delete,upsert,delete" {
		t.Fatalf("store calls %s", got)
	}
	if r := store.upserted[0].CountryRule; r.Issuer != "DE" || r.MinShare != NeighborShare {
		t.Fatalf("country rule %+v", r)
	}
	if store.upserted[0].Attrs["country"] != "DE" || store.upserted[8].Attrs["country"] != "PT" || store.upserted[0].Geometry == nil {
		t.Fatalf("country/geometry %+v", store.upserted[0])
	}
	var canceled int
	for _, id := range store.deleted {
		if id == "old-1" || id == "old-2" {
			canceled++
		}
	}
	if canceled != 2 || len(store.deleted) <= 2 {
		t.Fatalf("deleted %v", store.deleted)
	}
}

func TestPollAllCountriesFail(t *testing.T) {
	s, _ := newTestSource(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}, Country{"DE", "germany"}, Country{"FR", "france"})
	if _, err := s.Poll(t.Context(), domain.Validators{}); err == nil || !strings.Contains(err.Error(), "DE") || !strings.Contains(err.Error(), "FR") {
		t.Fatalf("err %v", err)
	}
}

func TestInfoCoverage(t *testing.T) {
	info := New(nil, nil, nil, nil).Info()
	if !info.OfficialWarnings || len(info.Coverage) != len(Countries) || info.Coverage[0] != "AT" || info.License != "CC BY 4.0" {
		t.Fatalf("%+v", info)
	}
}

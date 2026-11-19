package tsunami

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// Fixtures are real messages: the M8.7 Kamchatka earthquake of
// 2025-07-29 (Hawaii Watch → Warning → Advisory, Guam Watch → Advisory →
// Cancellation, PTWC threat forecasts), the M7.6 Panama earthquake of
// 2026-10-09 (final threat message), the M7.2 Vanuatu earthquake of
// 2026-10-24 (Hawaii information statement), and tsunami.gov's list of
// recent messages on 2026-11-19.
func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadCAP(t *testing.T, name string) Alert {
	t.Helper()
	a, err := ParseCAP(read(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var kamchatka = func(n int, product string) Entry {
	return Entry{
		Incident: "25210002", Center: "PHEB", Product: product, Bulletin: n,
		Path: fmt.Sprintf("/events/PHEB/2025/07/29/25210002/%d", n),
	}
}

func TestParseListing(t *testing.T) {
	entries, err := ParseListing(read(t, "previous.js"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 40 {
		t.Fatalf("%d entries, want the 40 listed", len(entries))
	}
	e := entries[0]
	if e.Key() != "PHEB-26322050-WEGM42" || e.Bulletin != 1 || e.Path != "/events/PHEB/2026/11/18/26322050/1" ||
		!e.Issued.Equal(time.Date(2026, 11, 18, 2, 30, 47, 0, time.UTC)) {
		t.Fatalf("first %+v", e)
	}
	// A path off the events tree, or a center that does not match it, is
	// never fetched.
	bad := `{'id' : 'x', 'issueTime' : '2026-11-18 02:30:47.0', 'URL' : '//evil.example/a', 'TWCID' : 'PHEB', 'WMOID' : 'WEGM42', 'bulletinNumber' : 1 },
	{'id' : 'y', 'issueTime' : '2026-11-18 02:30:47.0', 'URL' : '/events/PAAQ/2026/11/18/y/1', 'TWCID' : 'PHEB', 'WMOID' : 'WEGM42', 'bulletinNumber' : 1 }`
	if _, err := ParseListing([]byte(bad)); err == nil {
		t.Error("listing of unreadable entries accepted")
	}
}

func TestLatest(t *testing.T) {
	t0 := time.Date(2025, 7, 30, 0, 0, 0, 0, time.UTC)
	at := func(e Entry, h int) Entry { e.Issued = t0.Add(time.Duration(h) * time.Hour); return e }
	got := Latest([]Entry{
		at(kamchatka(3, "WEHW40"), 2), at(kamchatka(1, "WEHW40"), 0), at(kamchatka(2, "WEGM40"), 1),
		at(kamchatka(1, "WEPA40"), -30), // older than the window
	}, t0.Add(-24*time.Hour))
	if len(got) != 2 || got[0].Key() != "PHEB-25210002-WEGM40" || got[1].Bulletin != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestExpandUGC(t *testing.T) {
	for in, want := range map[string]string{
		"GUZ001>004-300135-":                   "GUZ001 GUZ002 GUZ003 GUZ004",
		"HIZ001-003-006>007-240439-":           "HIZ001 HIZ003 HIZ006 HIZ007",
		"AKZ145-WAZ001>002-ORC001-003-ORZ001-": "AKZ145 WAZ001 WAZ002 ORZ001", // counties skipped
	} {
		got, err := ExpandUGC(in)
		if err != nil || strings.Join(got, " ") != want {
			t.Errorf("%s: %v err %v, want %s", in, got, err, want)
		}
	}
	if _, err := ExpandUGC("HIZ001-0X3-"); err == nil {
		t.Error("malformed group accepted")
	}
}

func TestParseThreat(t *testing.T) {
	bands := ParseThreat(string(read(t, "kamchatka_3_WEPA40.txt")))
	if len(bands) != 3 {
		t.Fatalf("%d bands, want >3 m, 1–3 m, 0.3–1 m (under 0.3 m dropped)", len(bands))
	}
	if b := bands[0]; b.Level != severity.Extreme || strings.Join(b.Names, ",") != "Ecuador,Russia" || strings.Join(b.Countries, ",") != "EC,RU" {
		t.Errorf("over 3 m: %+v", b)
	}
	if b := bands[1]; b.Level != severity.Severe || strings.Join(b.Countries, ",") != "CL,US,JP,SB" || b.Names[3] != "Northwestern Hawaiian Islands" {
		t.Errorf("1–3 m: %+v", b) // Hawaii and the NW islands are both US
	}
	mid := bands[2]
	if mid.Level != severity.Moderate || len(mid.Unmatched) != 0 || !slices.Contains(mid.Countries, "FM") || !slices.Contains(mid.Countries, "UM") ||
		!slices.Contains(mid.Names, "Wallis and Futuna") || !slices.Contains(mid.Names, "Howland and Baker") || slices.Contains(mid.Countries, "AU") {
		t.Errorf("0.3–1 m: %+v", mid)
	}

	// Before a forecast exists: the coasts near the epicenter, no height.
	first := ParseThreat(string(read(t, "kamchatka_1_WEPA40.txt")))
	if len(first) != 1 || first[0].Label != "hazardous waves possible" || strings.Join(first[0].Countries, ",") != "RU,JP" {
		t.Errorf("first message %+v", first)
	}
	if v := ParseThreat(string(read(t, "vanuatu_1_WEPA40.txt"))); len(v) != 1 || v[0].Countries[0] != "VU" {
		t.Errorf("vanuatu %+v", v)
	}
	// The final message: the threat has passed.
	if final := ParseThreat(string(read(t, "panama_4_WEPA40.txt"))); len(final) != 0 {
		t.Errorf("final message %+v", final)
	}
}

func TestClassifyUSProducts(t *testing.T) {
	watch := loadCAP(t, "kamchatka_1_WEHW40.xml")
	out, err := Classify(kamchatka(1, "WEHW40"), &watch)
	if err != nil || out.Event == nil {
		t.Fatalf("%+v err %v", out, err)
	}
	ev := out.Event
	if ev.SourceEventID != "PHEB-25210002-WEHW40" || ev.Type != domain.TypeTsunami || ev.Kind != domain.KindWarning ||
		ev.Severity != "moderate" || ev.Title != "Tsunami Watch · Hawaii" || ev.SourceSeverity != "Tsunami Watch" {
		t.Errorf("watch %+v", ev)
	}
	if !strings.HasPrefix(ev.Summary, "Tsunami Watch for Hawaii after a preliminary magnitude ") ||
		!strings.Contains(ev.Summary, " earthquake off the east coast of Kamchatka, Russia.") {
		t.Errorf("summary %q", ev.Summary)
	}
	if len(out.Zones) < 20 || out.Zones[0] != "HIZ001" || ev.Attrs["center"] != "PTWC" || ev.Attrs["geometry"] != "epicenter" ||
		ev.URL != "https://www.tsunami.gov/events/PHEB/2025/07/29/25210002/1/WEHW40/WEHW40.txt" {
		t.Errorf("zones %v attrs %v url %s", out.Zones, ev.Attrs, ev.URL)
	}
	if exp, ok := ev.Attrs["expiresAt"].(time.Time); !ok || !exp.Equal(time.Date(2025, 7, 30, 2, 34, 1, 0, time.UTC)) {
		t.Errorf("expiresAt %v", ev.Attrs["expiresAt"])
	}
	if ev.StartedAt == nil || !ev.StartedAt.Equal(time.Date(2025, 7, 29, 23, 24, 52, 0, time.UTC)) {
		t.Errorf("origin %v", ev.StartedAt)
	}

	for name, want := range map[string]string{
		"kamchatka_3_WEHW40.xml":  "extreme", // Warning
		"kamchatka_10_WEHW40.xml": "severe",  // Advisory
		"kamchatka_3_WEGM40.xml":  "severe",  // Guam Advisory
	} {
		a := loadCAP(t, name)
		out, err := Classify(kamchatka(3, "WEHW40"), &a)
		if err != nil || out.Event == nil || out.Event.Severity != want {
			t.Errorf("%s: %+v err %v, want %s", name, out.Event, err, want)
		}
	}
	guam := loadCAP(t, "kamchatka_3_WEGM40.xml")
	if out, _ := Classify(kamchatka(3, "WEGM40"), &guam); out.Event.Title != "Tsunami Advisory · Guam and CNMI" {
		t.Errorf("guam title %q", out.Event.Title)
	}
}

func TestClassifyEnds(t *testing.T) {
	for name, at := range map[string]time.Time{
		"kamchatka_9_WEGM40.xml": time.Date(2025, 7, 30, 8, 40, 15, 0, time.UTC),  // Cancellation
		"panama_4_WEPA40.xml":    time.Date(2026, 10, 9, 19, 40, 6, 0, time.UTC),  // final: no info
		"vanuatu_1_WEHW42.xml":   time.Date(2026, 10, 24, 2, 39, 11, 0, time.UTC), // Information
	} {
		a := loadCAP(t, name)
		out, err := Classify(kamchatka(9, "WEGM40"), &a)
		if err != nil || !out.End || out.Event != nil {
			t.Errorf("%s: %+v err %v, want an end", name, out, err)
			continue
		}
		if !out.At.Equal(at) {
			t.Errorf("%s: ends at %v, want about %v", name, out.At, at)
		}
	}
	// Tests and exercises never reach the map.
	ex := loadCAP(t, "kamchatka_3_WEHW40.xml")
	ex.Status = "Exercise"
	if out, err := Classify(kamchatka(3, "WEHW40"), &ex); err != nil || out.Event != nil || out.End {
		t.Errorf("exercise %+v err %v", out, err)
	}
}

func TestThreat(t *testing.T) {
	a := loadCAP(t, "kamchatka_3_WEPA40.xml")
	out, err := Classify(kamchatka(3, "WEPA40"), &a)
	if err != nil || !out.NeedsText {
		t.Fatalf("%+v err %v, want the bulletin text", out, err)
	}
	out, err = Threat(kamchatka(3, "WEPA40"), &a, string(read(t, "kamchatka_3_WEPA40.txt")))
	if err != nil || out.Event == nil {
		t.Fatalf("%+v err %v", out, err)
	}
	ev := out.Event
	if ev.Kind != domain.KindForecast || ev.Severity != "extreme" || ev.Title != "Tsunami threat forecast · Ecuador +1" ||
		ev.Attrs["guidance"] != true || ev.CountryRule.Listed[0] != "EC" || !slices.Contains(ev.CountryRule.Listed, "NZ") {
		t.Errorf("forecast %+v", ev)
	}
	if !strings.HasPrefix(ev.Summary, "Guidance for national authorities, not a warning:") || !strings.Contains(ev.Summary, "over 3 m: Ecuador, Russia") {
		t.Errorf("summary %q", ev.Summary)
	}
	var g struct{ Type string }
	if err := json.Unmarshal(ev.Geometry, &g); err != nil || g.Type != "Point" {
		t.Errorf("geometry %s", ev.Geometry)
	}
}

// --- Poll ---

type fakeStore struct {
	upserted []domain.Event
	resolved []string
	expired  int
}

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	f.upserted = append(f.upserted, events...)
	return len(events), nil
}
func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }
func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	f.resolved = append(f.resolved, ids...)
	return len(ids), nil
}

func (f *fakeStore) ResolveExpired(context.Context, string, time.Time) (int, error) {
	f.expired++
	return 0, nil
}

type fakeZones struct{ asked [][]string }

func (f *fakeZones) Union(_ context.Context, ugcs []string) (json.RawMessage, []string, error) {
	f.asked = append(f.asked, ugcs)
	var found []string
	for _, u := range ugcs {
		if u != "HIZ054" { // one zone we do not hold
			found = append(found, u)
		}
	}
	return json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[-160,21],[-159,21],[-159,22],[-160,21]]]]}`), found, nil
}

func TestPoll(t *testing.T) {
	listing := `var prevEvents = { 'event' : [
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:43:46.0', 'URL' : '/events/PHEB/2025/07/29/25210002/3', 'TWCID' : 'PHEB', 'WMOID' : 'WEHW40', 'bulletinNumber' : 3 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:18:23.0', 'URL' : '/events/PHEB/2025/07/29/25210002/2', 'TWCID' : 'PHEB', 'WMOID' : 'WEHW40', 'bulletinNumber' : 2 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:43:00.0', 'URL' : '/events/PHEB/2025/07/29/25210002/3', 'TWCID' : 'PHEB', 'WMOID' : 'WEPA40', 'bulletinNumber' : 3 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 09:40:15.0', 'URL' : '/events/PHEB/2025/07/29/25210002/9', 'TWCID' : 'PHEB', 'WMOID' : 'WEGM40', 'bulletinNumber' : 9 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:50:00.0', 'URL' : '/events/PHEB/2025/07/29/25210002/4', 'TWCID' : 'PHEB', 'WMOID' : 'WEZS40', 'bulletinNumber' : 4 }
]};`
	files := map[string]string{
		"/events/js/previous.js":                                "",
		"/events/PHEB/2025/07/29/25210002/3/WEHW40/PHEBCAP.xml": "kamchatka_3_WEHW40.xml",
		"/events/PHEB/2025/07/29/25210002/3/WEPA40/PHEBCAP.xml": "kamchatka_3_WEPA40.xml",
		"/events/PHEB/2025/07/29/25210002/3/WEPA40/WEPA40.txt":  "kamchatka_3_WEPA40.txt",
		"/events/PHEB/2025/07/29/25210002/9/WEGM40/PHEBCAP.xml": "kamchatka_9_WEGM40.xml",
	}
	var fetched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = append(fetched, r.URL.Path)
		name, ok := files[r.URL.Path]
		switch {
		case !ok:
			w.WriteHeader(http.StatusNotFound) // WEZS40: unreadable, retried next poll
		case name == "":
			_, _ = io.WriteString(w, listing)
		default:
			_, _ = w.Write(read(t, name))
		}
	}))
	defer srv.Close()
	store, zones := &fakeStore{}, &fakeZones{}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, zones, slog.New(slog.DiscardHandler))
	s.base = srv.URL
	s.now = func() time.Time { return time.Date(2025, 7, 30, 10, 0, 0, 0, time.UTC) }

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	// Hawaii Warning (latest bulletin only) and the PTWC forecast stored;
	// Guam's cancellation ends its event; WEZS40 failed to fetch.
	// Oldest first: the forecast (00:43:00) before Hawaii (00:43:46).
	if len(store.upserted) != 2 || store.upserted[0].Kind != domain.KindForecast || store.upserted[1].SourceEventID != "PHEB-25210002-WEHW40" {
		t.Fatalf("upserted %+v", store.upserted)
	}
	hi := store.upserted[1]
	if hi.Severity != "extreme" || hi.Attrs["geometry"] != "zones" || hi.CountryRule.MinShare != 0.05 ||
		fmt.Sprint(hi.Attrs["zonesUnresolved"]) != "[HIZ054]" || !strings.Contains(string(hi.Geometry), "MultiPolygon") {
		t.Errorf("hawaii %+v", hi)
	}
	if fmt.Sprint(store.resolved) != "[PHEB-25210002-WEGM40]" || store.expired != 1 || res.Upserted != 3 {
		t.Errorf("resolved %v expired %d result %+v", store.resolved, store.expired, res)
	}
	if slices.Contains(fetched, "/events/PHEB/2025/07/29/25210002/2/WEHW40/PHEBCAP.xml") {
		t.Error("an older bulletin of a product was fetched")
	}

	// Next poll: handled messages are not fetched again; the failed one is.
	fetched = nil
	if _, err := s.Poll(t.Context(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 2 || fetched[1] != "/events/PHEB/2025/07/29/25210002/4/WEZS40/PHEBCAP.xml" || len(store.upserted) != 2 {
		t.Errorf("second poll fetched %v, upserted %d", fetched, len(store.upserted))
	}
}

func TestInfo(t *testing.T) {
	info := New(nil, nil, nil, nil).Info()
	if info.ID != ID || info.PollInterval != time.Minute || !info.OfficialWarnings || !slices.Contains(info.Coverage, "GU") {
		t.Fatalf("%+v", info)
	}
}

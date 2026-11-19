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
// 2025-07-29 (Hawaii Watch → Warning → Advisory → Cancellation, Guam
// Advisory → Cancellation, PTWC threat forecasts, NTWC's West Coast and
// Alaska product with mixed segments → Cancellation), the M7.6 Panama earthquake of
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

func threat(t *testing.T, name string) []Band {
	t.Helper()
	b, ok := ParseThreat(string(read(t, name)))
	if !ok {
		t.Fatalf("%s: no forecast section", name)
	}
	return b
}

func TestParseThreat(t *testing.T) {
	bands := threat(t, "kamchatka_3_WEPA40.txt")
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
	first := threat(t, "kamchatka_1_WEPA40.txt")
	if len(first) != 1 || first[0].Label != "hazardous waves possible" || strings.Join(first[0].Countries, ",") != "RU,JP" {
		t.Errorf("first message %+v", first)
	}
	if v := threat(t, "vanuatu_1_WEPA40.txt"); len(v) != 1 || v[0].Countries[0] != "VU" {
		t.Errorf("vanuatu %+v", v)
	}
	// A bulletin with no forecast section is not "no threat".
	if _, ok := ParseThreat("PTWC BULLETIN\nNOTHING HERE\n"); ok {
		t.Error("text without a forecast section read as a forecast")
	}
}

// one classifies a message expected to hold exactly one event.
func one(t *testing.T, e Entry, name string) (domain.Event, []string) {
	t.Helper()
	a := loadCAP(t, name)
	out, err := Classify(e, &a)
	if err != nil || len(out.Events) != 1 {
		t.Fatalf("%s: %+v err %v, want one event", name, out, err)
	}
	return out.Events[0].Event, out.Events[0].Zones
}

func TestClassifyUSProducts(t *testing.T) {
	ev, zones := one(t, kamchatka(1, "WEHW40"), "kamchatka_1_WEHW40.xml")
	if ev.SourceEventID != "PHEB-25210002-WEHW40-watch" || ev.Type != domain.TypeTsunami || ev.Kind != domain.KindWarning ||
		ev.Severity != "moderate" || ev.Title != "Tsunami Watch · Hawaii" || ev.SourceSeverity != "Tsunami Watch" {
		t.Errorf("watch %+v", ev)
	}
	if !strings.HasPrefix(ev.Summary, "Tsunami Watch for Hawaii after a preliminary magnitude ") ||
		!strings.Contains(ev.Summary, " earthquake off the east coast of Kamchatka, Russia.") {
		t.Errorf("summary %q", ev.Summary)
	}
	if len(zones) < 20 || zones[0] != "HIZ001" || ev.Attrs["center"] != "PTWC" || ev.Attrs["geometry"] != "epicenter" ||
		ev.URL != "https://www.tsunami.gov/events/PHEB/2025/07/29/25210002/1/WEHW40/WEHW40.txt" {
		t.Errorf("zones %v attrs %v url %s", zones, ev.Attrs, ev.URL)
	}
	if exp, ok := ev.Attrs["expiresAt"].(time.Time); !ok || !exp.Equal(time.Date(2025, 7, 30, 2, 34, 1, 0, time.UTC)) {
		t.Errorf("expiresAt %v", ev.Attrs["expiresAt"])
	}
	if ev.StartedAt == nil || !ev.StartedAt.Equal(time.Date(2025, 7, 29, 23, 24, 52, 0, time.UTC)) {
		t.Errorf("origin %v", ev.StartedAt)
	}

	for name, want := range map[string]string{
		"kamchatka_3_WEHW40.xml":  "extreme warning",
		"kamchatka_10_WEHW40.xml": "severe advisory",
	} {
		ev, _ := one(t, kamchatka(3, "WEHW40"), name)
		if got := ev.Severity + " " + strings.TrimPrefix(ev.SourceEventID, "PHEB-25210002-WEHW40-"); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
	// 2025's Guam product still numbered Rota, Tinian and Saipan GUZ002-004;
	// they are MPZ001-003 in the 2026 zone file, and PTWC now sends those
	// (2026-11-18: "GUZ001-MPZ001>003-180430-").
	guam, _ := one(t, kamchatka(3, "WEGM40"), "kamchatka_3_WEGM40.xml")
	if guam.Severity != "severe" || guam.Title != "Tsunami Advisory · Guam" {
		t.Errorf("guam %s %q", guam.Severity, guam.Title)
	}
	zones, err := ExpandUGC("GUZ001-MPZ001>003-180430-")
	if err != nil || regions(zones) != "Guam, Northern Mariana Islands" || len(zones) != 4 || fmt.Sprint(zoneCountries(zones)) != "[GU MP]" {
		t.Errorf("current Guam zones %v %q err %v", zones, regions(zones), err)
	}
}

var ntwc = func(n int) Entry {
	return Entry{
		Incident: "t06p1k", Center: "PAAQ", Product: "WEAK51", Bulletin: n,
		Path: fmt.Sprintf("/events/PAAQ/2025/07/29/t06p1k/%d", n),
	}
}

// NTWC's product carries one <info> per coastal segment. Bulletin 3 leads
// with a canceled segment while Warning and Advisory segments stay in
// force; bulletin 5 has two Warning segments on opposite coasts.
func TestClassifyNTWCSegments(t *testing.T) {
	a := loadCAP(t, "kamchatka_ntwc_3_WEAK51.xml")
	out, err := Classify(ntwc(3), &a)
	if err != nil || len(out.Events) != 2 {
		t.Fatalf("%+v err %v, want a warning and an advisory", out, err)
	}
	warn, adv := out.Events[0], out.Events[1]
	if warn.Event.SourceEventID != "PAAQ-t06p1k-WEAK51-warning" || warn.Event.Severity != "extreme" ||
		strings.Join(warn.Zones, ",") != "AKZ787,AKZ795,AKZ791" || warn.Event.Title != "Tsunami Warning · Alaska" ||
		warn.Event.Attrs["center"] != "NTWC" {
		t.Errorf("warning %+v zones %v", warn.Event, warn.Zones)
	}
	if !strings.HasPrefix(warn.Event.Summary, "Tsunami Warning for Samalga Pass, Alaska (30 miles SW of Nikolski) to Attu, Alaska after ") {
		t.Errorf("warning summary %q", warn.Event.Summary)
	}
	if adv.Event.SourceEventID != "PAAQ-t06p1k-WEAK51-advisory" || adv.Event.Severity != "severe" ||
		!slices.Contains(adv.Zones, "CAZ043") || !slices.Contains(adv.Zones, "AKZ781") || slices.Contains(adv.Zones, "AKZ328") ||
		adv.Event.Title != "Tsunami Advisory · California, Oregon, Washington, British Columbia, Alaska" ||
		fmt.Sprint(adv.Event.CountryRule.Listed) != "[US CA]" ||
		len(adv.Event.Attrs["areas"].([]string)) != 2 {
		t.Errorf("advisory %+v zones %v", adv.Event, adv.Zones) // AKZ328: the canceled segment
	}

	b := loadCAP(t, "kamchatka_ntwc_5_WEAK51.xml")
	out, err = Classify(ntwc(5), &b)
	if err != nil || len(out.Events) != 2 {
		t.Fatalf("%+v err %v", out, err)
	}
	if w := out.Events[0]; !slices.Contains(w.Zones, "CAZ103") || !slices.Contains(w.Zones, "AKZ787") || w.Event.Title != "Tsunami Warning · California, Alaska" {
		t.Errorf("two warning segments %q zones %v", w.Event.Title, w.Zones)
	}

	// The first message: one Advisory segment.
	first, _ := one(t, ntwc(1), "kamchatka_ntwc_1_WEAK51.xml")
	if first.SourceEventID != "PAAQ-t06p1k-WEAK51-advisory" {
		t.Errorf("first %s", first.SourceEventID)
	}
}

func TestClassifyEnds(t *testing.T) {
	for name, at := range map[string]time.Time{
		"kamchatka_9_WEGM40.xml":       time.Date(2025, 7, 30, 8, 40, 15, 0, time.UTC),  // Cancellation
		"kamchatka_20_WEHW40.xml":      time.Date(2025, 7, 30, 18, 59, 40, 0, time.UTC), // Hawaii's Cancellation
		"kamchatka_ntwc_35_WEAK51.xml": time.Date(2025, 7, 31, 14, 17, 23, 0, time.UTC), // NTWC's last segment canceled
		"panama_4_WEPA40.xml":          time.Date(2026, 10, 9, 19, 40, 6, 0, time.UTC),  // final: no info
		"vanuatu_1_WEHW42.xml":         time.Date(2026, 10, 24, 2, 39, 11, 0, time.UTC), // Information
	} {
		a := loadCAP(t, name)
		out, err := Classify(kamchatka(9, "WEGM40"), &a)
		if err != nil || len(out.Events) != 0 || out.Ignore || out.NeedsText {
			t.Errorf("%s: %+v err %v, want an end", name, out, err)
			continue
		}
		if !out.At.Equal(at) {
			t.Errorf("%s: ends at %v, want %v", name, out.At, at)
		}
	}
	// Tests and exercises never reach the map.
	ex := loadCAP(t, "kamchatka_3_WEHW40.xml")
	ex.Status = "Exercise"
	if out, err := Classify(kamchatka(3, "WEHW40"), &ex); err != nil || !out.Ignore {
		t.Errorf("exercise %+v err %v", out, err)
	}
}

// Wording Shield does not know is an error, retried and counted, never
// read as the end of a warning.
func TestClassifyFailsClosed(t *testing.T) {
	renamed := loadCAP(t, "kamchatka_3_WEHW40.xml")
	renamed.Info[0].Event = "Tsunami Danger"
	if _, err := Classify(kamchatka(3, "WEHW40"), &renamed); err == nil {
		t.Error("unknown US alert accepted")
	}
	elsewhere := loadCAP(t, "kamchatka_3_WEPA40.xml")
	if _, err := Classify(kamchatka(3, "WEXX40"), &elsewhere); err == nil {
		t.Error("threat-like message from an unknown product accepted")
	}
	a := loadCAP(t, "kamchatka_3_WEPA40.xml")
	if _, err := Threat(kamchatka(3, "WEPA40"), &a, "PTWC BULLETIN\n"); err == nil {
		t.Error("threat message without a forecast ended the forecast")
	}
}

func TestThreat(t *testing.T) {
	a := loadCAP(t, "kamchatka_3_WEPA40.xml")
	out, err := Classify(kamchatka(3, "WEPA40"), &a)
	if err != nil || !out.NeedsText {
		t.Fatalf("%+v err %v, want the bulletin text", out, err)
	}
	out, err = Threat(kamchatka(3, "WEPA40"), &a, string(read(t, "kamchatka_3_WEPA40.txt")))
	if err != nil || len(out.Events) != 1 {
		t.Fatalf("%+v err %v", out, err)
	}
	ev := out.Events[0].Event
	if ev.SourceEventID != "PHEB-25210002-WEPA40" || ev.Kind != domain.KindForecast || ev.Severity != "extreme" ||
		ev.Title != "Tsunami threat forecast · Ecuador +1" || ev.Attrs["guidance"] != true ||
		ev.CountryRule.Listed[0] != "EC" || !slices.Contains(ev.CountryRule.Listed, "NZ") {
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
	active   map[string]bool
	upserted []domain.Event
	resolved []string
	expired  int
}

func newFakeStore() *fakeStore { return &fakeStore{active: map[string]bool{}} }

func (f *fakeStore) Upsert(_ context.Context, _ string, events []domain.Event) (int, error) {
	for i := range events {
		f.active[events[i].SourceEventID] = true
	}
	f.upserted = append(f.upserted, events...)
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

// Resolve ends only events that are active, as the store does.
func (f *fakeStore) Resolve(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	n := 0
	for _, id := range ids {
		if f.active[id] {
			delete(f.active, id)
			f.resolved = append(f.resolved, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) ResolveExpired(context.Context, string, time.Time) (int, error) {
	f.expired++
	return 0, nil
}

type fakeZones struct{}

func (fakeZones) Union(_ context.Context, ugcs []string) (json.RawMessage, []string, error) {
	var found []string
	for _, u := range ugcs {
		if u != "HIZ054" { // one zone we do not hold
			found = append(found, u)
		}
	}
	return json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[-160,21],[-159,21],[-159,22],[-160,21]]]]}`), found, nil
}

const lastModified = "Wed, 30 Jul 2025 00:50:00 GMT"

func TestPoll(t *testing.T) {
	listing := `var prevEvents = { 'event' : [
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:43:46.0', 'URL' : '/events/PHEB/2025/07/29/25210002/3', 'TWCID' : 'PHEB', 'WMOID' : 'WEHW40', 'bulletinNumber' : 3 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:18:23.0', 'URL' : '/events/PHEB/2025/07/29/25210002/2', 'TWCID' : 'PHEB', 'WMOID' : 'WEHW40', 'bulletinNumber' : 2 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:43:00.0', 'URL' : '/events/PHEB/2025/07/29/25210002/3', 'TWCID' : 'PHEB', 'WMOID' : 'WEPA40', 'bulletinNumber' : 3 },
{ 'id' : '25210002', 'issueTime' : '2025-07-30 00:50:00.0', 'URL' : '/events/PHEB/2025/07/29/25210002/4', 'TWCID' : 'PHEB', 'WMOID' : 'WEZS40', 'bulletinNumber' : 4 }
]};`
	files := map[string]string{
		"/events/PHEB/2025/07/29/25210002/3/WEHW40/PHEBCAP.xml": "kamchatka_3_WEHW40.xml",
		"/events/PHEB/2025/07/29/25210002/3/WEPA40/PHEBCAP.xml": "kamchatka_3_WEPA40.xml",
		"/events/PHEB/2025/07/29/25210002/3/WEPA40/WEPA40.txt":  "kamchatka_3_WEPA40.txt",
	}
	var fetched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = append(fetched, r.URL.Path)
		if r.URL.Path == ListPath {
			if r.Header.Get("If-Modified-Since") == lastModified {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("Last-Modified", lastModified)
			_, _ = io.WriteString(w, listing)
			return
		}
		name, ok := files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound) // WEZS40 until it is published
			return
		}
		_, _ = w.Write(read(t, name))
	}))
	defer srv.Close()
	store := newFakeStore()
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, fakeZones{}, slog.New(slog.DiscardHandler))
	s.base = srv.URL
	s.now = func() time.Time { return time.Date(2025, 7, 30, 1, 0, 0, 0, time.UTC) }

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	// The PTWC forecast (00:43:00) and the Hawaii Warning (00:43:46, the
	// latest bulletin only) are stored, oldest first; WEZS40 failed.
	if len(store.upserted) != 2 || store.upserted[0].Kind != domain.KindForecast || store.upserted[1].SourceEventID != "PHEB-25210002-WEHW40-warning" {
		t.Fatalf("upserted %+v", store.upserted)
	}
	hi := store.upserted[1]
	if hi.Severity != "extreme" || hi.Attrs["geometry"] != "zones" || fmt.Sprint(hi.CountryRule.Listed) != "[US]" ||
		fmt.Sprint(hi.Attrs["zonesUnresolved"]) != "[HIZ054]" || !strings.Contains(string(hi.Geometry), "MultiPolygon") {
		t.Errorf("hawaii %+v", hi)
	}
	if store.expired != 1 || res.Upserted != 2 {
		t.Errorf("expired %d result %+v", store.expired, res)
	}
	if slices.Contains(fetched, "/events/PHEB/2025/07/29/25210002/2/WEHW40/PHEBCAP.xml") {
		t.Error("an older bulletin of a product was fetched")
	}
	// A message failed, so the list's validators must not advance: a 304
	// next poll would hide it until the list changed.
	if res.Validators.LastModified != "" {
		t.Errorf("validators advanced past a failed message: %+v", res.Validators)
	}

	// Next poll: the list is read again; handled messages are not
	// fetched, the failed one is, and now succeeds.
	files["/events/PHEB/2025/07/29/25210002/4/WEZS40/PHEBCAP.xml"] = "kamchatka_ntwc_35_WEAK51.xml" // any ending message
	fetched = nil
	res, err = s.Poll(t.Context(), res.Validators)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fetched, " ") != ListPath+" /events/PHEB/2025/07/29/25210002/4/WEZS40/PHEBCAP.xml" || res.Validators.LastModified != lastModified {
		t.Errorf("second poll fetched %v, validators %+v", fetched, res.Validators)
	}
	// Then the list is unchanged: 304, nothing fetched.
	fetched = nil
	if res, err = s.Poll(t.Context(), res.Validators); err != nil || !res.NotModified || len(fetched) != 1 {
		t.Errorf("third poll %+v err %v fetched %v", res, err, fetched)
	}
}

// A message is a snapshot of its product: the levels it holds are stored,
// the product's others end, and a level already past its expiry (a stale
// bulletin read after a restart) ends instead of being stored as active.
func TestApplySnapshot(t *testing.T) {
	store := newFakeStore()
	s := New(nil, store, fakeZones{}, slog.New(slog.DiscardHandler))
	e := kamchatka(3, "WEHW40")
	store.active["PHEB-25210002-WEHW40-watch"] = true // bulletin 2

	a := loadCAP(t, "kamchatka_3_WEHW40.xml") // the Watch became a Warning
	out, err := Classify(e, &a)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Date(2025, 7, 30, 1, 0, 0, 0, time.UTC)
	if _, err := s.apply(t.Context(), e, out, before); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(store.resolved) != "[PHEB-25210002-WEHW40-watch]" || !store.active["PHEB-25210002-WEHW40-warning"] {
		t.Errorf("upgrade: resolved %v active %v", store.resolved, store.active)
	}

	stale := newFakeStore()
	stale.active["PHEB-25210002-WEHW40-warning"] = true
	s.store = stale
	after := time.Date(2025, 7, 30, 4, 0, 0, 0, time.UTC) // past 03:43:46
	if _, err := s.apply(t.Context(), e, out, after); err != nil {
		t.Fatal(err)
	}
	if len(stale.upserted) != 0 || fmt.Sprint(stale.resolved) != "[PHEB-25210002-WEHW40-warning]" {
		t.Errorf("expired: upserted %d resolved %v", len(stale.upserted), stale.resolved)
	}
}

func TestInfo(t *testing.T) {
	info := New(nil, nil, nil, nil).Info()
	if info.ID != ID || info.PollInterval != time.Minute || !info.OfficialWarnings || !slices.Contains(info.Coverage, "GU") {
		t.Fatalf("%+v", info)
	}
}

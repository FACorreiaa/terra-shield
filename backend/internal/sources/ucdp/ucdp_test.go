package ucdp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

// fixture is eight real September 2026 rows with the news-source columns
// blanked.
func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/GEDEvent_v26_0_9.csv")
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

// The fixture is eight real September 2026 rows with the news-source
// columns blanked.
func TestParse(t *testing.T) {
	events, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	m := byID(events)
	// 644962 is located only to its country, 642206 at sea: skipped.
	if len(events) != 6 || m["644962"].SourceEventID != "" || m["642206"].SourceEventID != "" {
		t.Fatalf("got %d events: %v", len(events), m)
	}

	mali := m["640181"]
	if mali.Type != domain.TypeConflict || mali.Kind != domain.KindObserved || mali.Severity != "extreme" ||
		mali.SourceSeverity != "105 deaths (105–150)" || mali.Title != "Government of Mali – JNIM" ||
		mali.URL != "https://ucdp.uu.se/country/432" || mali.Attrs["severityProvisional"] != true ||
		mali.Attrs["violenceType"] != "state_based" || mali.Attrs["locationPrecision"] != "exact" ||
		mali.Attrs["deathsBest"] != 105 || mali.Attrs["expiresAt"] != "2026-12-09T00:00:00Z" {
		t.Errorf("mali %+v", mali)
	}
	if !mali.ObservedAt.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) || !mali.UpdatedAt.Equal(mali.ObservedAt) || !mali.StartedAt.Equal(mali.ObservedAt) {
		t.Errorf("mali dates %v %v %v", mali.StartedAt, mali.ObservedAt, mali.UpdatedAt)
	}

	if afc := m["644306"]; afc.Severity != "moderate" || afc.SourceSeverity != "7 deaths" || afc.Attrs["severityProvisional"] != nil ||
		afc.Attrs["violenceType"] != "non_state" || afc.Attrs["datePrecision"] != "days_2_6" {
		t.Errorf("afc %+v", afc)
	}
	if unnamed := m["645075"]; unnamed.Title != "Unidentified group – Civilians" || unnamed.Attrs["sideA"] != "Unidentified group" ||
		unnamed.Attrs["violenceType"] != "one_sided" || unnamed.SourceSeverity != "1 death" {
		t.Errorf("unnamed actor %+v", unnamed)
	}
	if long := m["645393"]; long.Attrs["locationPrecision"] != "admin1" || long.Attrs["datePrecision"] != "months" ||
		!long.StartedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || long.Attrs["severityProvisional"] != true {
		t.Errorf("long event %+v", long)
	}
	if zero := m["639918"]; zero.Severity != "minor" || zero.SourceSeverity != "0 deaths (0–1)" {
		t.Errorf("zero %+v", zero)
	}
	for _, e := range events {
		for k := range e.Attrs {
			if strings.HasPrefix(strings.ToLower(k), "source") && k != "sourceCount" {
				t.Errorf("%s stores %s", e.SourceEventID, k)
			}
		}
	}
}

func TestParseFailsClosed(t *testing.T) {
	good := string(fixture(t))
	header, rows, _ := strings.Cut(good, "\r\n")
	first, _, _ := strings.Cut(rows, "\r\n")
	for name, body := range map[string]string{
		"missing column": strings.Replace(header, "where_prec", "precision", 1) + "\r\n" + first,
		"bad violence":   header + "\r\n" + strings.Replace(first, ",Check deaths,1,", ",Check deaths,9,", 1),
		"short record":   header + "\r\n" + first[:200],
		"empty":          "",
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMonthsAndFileName(t *testing.T) {
	got := months(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	var names []string
	for _, m := range got {
		names = append(names, FileName(m.Year(), m.Month()))
	}
	if want := []string{"GEDEvent_v26_0_8.csv", "GEDEvent_v26_0_9.csv", "GEDEvent_v26_0_10.csv"}; !slices.Equal(names, want) {
		t.Fatalf("%v, want %v", names, want)
	}
	if n := FileName(2025, time.December); n != "GEDEvent_v25_0_12.csv" {
		t.Fatal(n)
	}
	jan := months(time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC))
	if jan[0].Month() != time.October || jan[len(jan)-1] != time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("across the year %v", jan)
	}
}

type fakeStore struct {
	batches  [][]domain.Event
	sweptAt  time.Time
	expiries int
}

func (f *fakeStore) Upsert(_ context.Context, sourceID string, events []domain.Event) (int, error) {
	if sourceID != ID {
		panic(sourceID)
	}
	f.batches = append(f.batches, events)
	return len(events), nil
}

func (*fakeStore) Delete(context.Context, string, []string) (int, error) { return 0, nil }

func (f *fakeStore) ResolveExpired(_ context.Context, _ string, now time.Time) (int, error) {
	f.sweptAt = now
	return f.expiries, nil
}

var _ Store = (*fakeStore)(nil)

func serve(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Last-Modified", "Tue, 20 Oct 2026 20:37:37 GMT")
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPoll(t *testing.T) {
	// October and November are not released yet (404).
	srv := serve(t, map[string][]byte{"GEDEvent_v26_0_9.csv": fixture(t)})

	// 2026-12-10: events that ended by 2026-09-10 (Mali, 639918, 645075)
	// are past their 90 days; Brazil (ended 2026-09-14) is not. The window
	// starts in September, so August is not read.
	now := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{expiries: 2}
	s := &Source{base: srv.URL + "/", client: fetch.New(fetch.Options{HostGap: time.Millisecond}), store: store, now: func() time.Time { return now }}
	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if !store.sweptAt.Equal(now) {
		t.Errorf("swept at %v", store.sweptAt)
	}
	var got []string
	for _, b := range store.batches {
		for _, e := range b {
			got = append(got, e.SourceEventID)
		}
	}
	if !slices.Equal(got, []string{"644306", "645393", "645598"}) {
		t.Fatalf("stored %v", got)
	}
	if res.Upserted != 3+2 || !res.SourceUpdatedAt.Equal(time.Date(2026, 10, 20, 20, 37, 37, 0, time.UTC)) {
		t.Fatalf("result %+v", res)
	}
}

func TestPollNewerMonthWinsAndBatches(t *testing.T) {
	fixture := fixture(t)
	header, rows, _ := strings.Cut(string(fixture), "\r\n")
	mali, _, _ := strings.Cut(rows, "\r\n")
	// 450 copies of one row under new ids: three batches.
	var b strings.Builder
	b.WriteString(header + "\r\n")
	for i := range 450 {
		b.WriteString(strings.Replace(mali, "640181,", strconv.Itoa(900000+i)+",", 1) + "\r\n")
	}
	august := header + "\r\n" + strings.Replace(mali, "Check deaths", "Clear", 1) + "\r\n"
	srv := serve(t, map[string][]byte{"GEDEvent_v26_0_8.csv": []byte(august), "GEDEvent_v26_0_9.csv": fixture, "GEDEvent_v26_0_10.csv": []byte(b.String())})

	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	s := &Source{base: srv.URL + "/", client: fetch.New(fetch.Options{HostGap: time.Millisecond}), store: store, now: func() time.Time { return now }}
	if _, err := s.Poll(t.Context(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	var sizes []int
	all := map[string]domain.Event{}
	for _, batch := range store.batches {
		sizes = append(sizes, len(batch))
		for _, e := range batch {
			all[e.SourceEventID] = e
		}
	}
	if !slices.Equal(sizes, []int{200, 200, 56}) {
		t.Errorf("batches %v", sizes)
	}
	if all["640181"].Attrs["codeStatus"] != "Check deaths" {
		t.Errorf("September's copy should win over August's: %v", all["640181"].Attrs["codeStatus"])
	}
}

func TestPollNoReleaseFails(t *testing.T) {
	srv := serve(t, nil)
	s := &Source{base: srv.URL + "/", client: fetch.New(fetch.Options{HostGap: time.Millisecond}), store: &fakeStore{}, now: time.Now}
	if _, err := s.Poll(t.Context(), domain.Validators{}); err == nil || !strings.Contains(err.Error(), "no monthly release") {
		t.Fatalf("err %v", err)
	}
}

func TestInfo(t *testing.T) {
	info := (&Source{}).Info()
	if info.ID != ID || info.License != "CC BY 4.0" || info.Attribution == "" || !info.Enabled || info.PollInterval != 6*time.Hour {
		t.Fatalf("%+v", info)
	}
}

package firms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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

// TestParseRecordedFile: 40 rows sampled from the real NOAA-20 file of
// 2026-11-16.
func TestParseRecordedFile(t *testing.T) {
	fires, err := Parse(strings.NewReader(string(read(t, "J1_VIIRS_C2_Global_24h.csv"))), "NOAA-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(fires) != 40 {
		t.Fatalf("%d detections, want 40", len(fires))
	}
	f := fires[0]
	want := time.Date(2026, 11, 15, 0, 11, 0, 0, time.UTC)
	if f.Satellite != "NOAA-20" || !f.AcquiredAt.Equal(want) || f.Lat != 44.83165 || f.Lon != 20.69508 ||
		f.Confidence != "nominal" || f.FRP == nil || *f.FRP != 0.66 {
		t.Fatalf("first row %+v", f)
	}
}

func TestParseEdgeCases(t *testing.T) {
	const in = "latitude,longitude,acq_date,acq_time,confidence,frp,extra\n" +
		"10.5,20.5,2026-11-16,11,high,,x\n" + // short time pads to 00:11; empty FRP
		"95,20,2026-11-16,0100,low,1,x\n" + // latitude out of range
		"10,20,not-a-date,0100,low,1,x\n" + // bad date
		"abc,20,2026-11-16,0100,low,1,x\n" // bad number
	fires, err := Parse(strings.NewReader(in), "NOAA-21")
	if err != nil || len(fires) != 1 {
		t.Fatalf("%+v err %v", fires, err)
	}
	if !fires[0].AcquiredAt.Equal(time.Date(2026, 11, 16, 0, 11, 0, 0, time.UTC)) || fires[0].FRP != nil {
		t.Fatalf("%+v", fires[0])
	}

	if _, err := Parse(strings.NewReader("latitude,longitude\n1,2\n"), "x"); err == nil {
		t.Fatal("missing columns accepted")
	}
	if fires, err := Parse(strings.NewReader(""), "x"); err != nil || fires != nil {
		t.Fatalf("empty file %v %v", fires, err)
	}
}

type fakeFires struct {
	inserted [][]domain.FireDetection
	purged   time.Time
}

func (f *fakeFires) Insert(_ context.Context, fires []domain.FireDetection) (int, error) {
	f.inserted = append(f.inserted, fires)
	return len(fires), nil
}

func (f *fakeFires) PurgeBefore(_ context.Context, t time.Time) (int, error) {
	f.purged = t
	return 0, nil
}

var _ ingest.FireStore = (*fakeFires)(nil)

// Two files; the second poll HEADs both, sees one changed, and downloads
// only that one.
func TestPollDownloadsOnlyChangedFiles(t *testing.T) {
	body := read(t, "J1_VIIRS_C2_Global_24h.csv")
	etags := map[string]string{"/a.csv": `"a1"`, "/b.csv": `"b1"`}
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", etags[r.URL.Path])
		if r.Method == http.MethodGet {
			gets.Add(1)
			_, _ = w.Write(body)
		}
	}))
	defer srv.Close()
	store := &fakeFires{}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.files = []File{{"NOAA-20", srv.URL + "/a.csv"}, {"NOAA-21", srv.URL + "/b.csv"}}

	res, err := s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.NotModified || res.Upserted != 80 || gets.Load() != 2 {
		t.Fatalf("first %+v err %v gets %d", res, err, gets.Load())
	}

	res, err = s.Poll(t.Context(), domain.Validators{})
	if err != nil || !res.NotModified || gets.Load() != 2 {
		t.Fatalf("unchanged %+v err %v gets %d", res, err, gets.Load())
	}

	etags["/b.csv"] = `"b2"`
	res, err = s.Poll(t.Context(), domain.Validators{})
	if err != nil || res.NotModified || res.Upserted != 40 || gets.Load() != 3 {
		t.Fatalf("one changed %+v err %v gets %d", res, err, gets.Load())
	}
	if last := store.inserted[len(store.inserted)-1]; last[0].Satellite != "NOAA-21" {
		t.Fatalf("inserted %s rows", last[0].Satellite)
	}
}

func TestPollFailureDoesNotAdvanceValidators(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("latitude,longitude,acq_date,acq_time,confidence,frp\n"))
	}))
	defer srv.Close()
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), &fakeFires{})
	s.files = []File{{"NOAA-20", srv.URL}}

	fail.Store(true)
	if _, err := s.Poll(t.Context(), domain.Validators{}); err == nil || !strings.Contains(err.Error(), "NOAA-20") {
		t.Fatalf("err %v", err)
	}
	if len(s.held) != 0 {
		t.Fatal("validators advanced on failure")
	}
}

func TestHousekeep(t *testing.T) {
	store := &fakeFires{}
	s := New(nil, store)
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if err := s.Housekeep(t.Context()); err != nil || !store.purged.Equal(now.Add(-48*time.Hour)) {
		t.Fatalf("purged before %s err %v", store.purged, err)
	}
	if s.HousekeepInterval() != time.Hour || s.Info().PollInterval != 15*time.Minute {
		t.Fatal("intervals")
	}
}

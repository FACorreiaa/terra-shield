package aircraft

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shield/internal/domain"
)

var t0 = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// fakeFeed answers every circle with the aircraft inside it, as the feed
// at the current clock, or with err.
type fakeFeed struct {
	clock     *clock
	sky       []domain.Aircraft
	calls     atomic.Int32
	mu        sync.Mutex
	err       error
	remaining int
	gate      chan struct{} // when set, Area waits on it
}

func (f *fakeFeed) Area(ctx context.Context, lat, lon float64, radiusNM int) (domain.AircraftArea, error) {
	f.calls.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return domain.AircraftArea{}, f.err
	}
	var in []domain.Aircraft
	for i := range f.sky {
		if distanceNM(lat, lon, f.sky[i].Lat, f.sky[i].Lon) <= float64(radiusNM) {
			in = append(in, f.sky[i])
		}
	}
	return domain.AircraftArea{Aircraft: in, At: f.clock.now(), QuotaRemaining: f.remaining}, nil
}

func (f *fakeFeed) fail(err error) { f.mu.Lock(); f.err = err; f.mu.Unlock() }

func setup(o Options, sky ...domain.Aircraft) (*Service, *fakeFeed, *clock) {
	c := &clock{t: t0}
	f := &fakeFeed{clock: c, sky: sky, remaining: -1}
	o.Now = c.now
	return New(f, o), f, c
}

func plane(hex string, lon, lat float64, at time.Time) domain.Aircraft {
	return domain.Aircraft{Hex: hex, Lon: lon, Lat: lat, PositionAt: at}
}

// Lisbon to Madrid: 2 × 2 cells (-10..0, 35..45).
var iberia = domain.BBox{MinLon: -9.9, MinLat: 36, MaxLon: -0.1, MaxLat: 44}

func TestRadiusCoversEveryCell(t *testing.T) {
	for i := range latCells {
		for j := range lonCells {
			r := radiusNM(cellKey{i, j})
			if r < 1 || r > 250 {
				t.Fatalf("cell %d,%d radius %d NM is outside the feed's 1–250", i, j, r)
			}
		}
	}
	if r := radiusNM(cellKey{latCells / 2, 0}); r != 213 {
		t.Errorf("equator cell radius %d, want 213 (farthest corner 212.1 NM)", r)
	}
}

func TestCells(t *testing.T) {
	cases := []struct {
		name string
		bb   domain.BBox
		want int
		err  bool
	}{
		{"inside one cell", domain.BBox{MinLon: -9.3, MinLat: 38.6, MaxLon: -9.0, MaxLat: 38.8}, 1, false},
		{"iberia", iberia, 4, false},
		{"across the antimeridian", domain.BBox{MinLon: 178, MinLat: -18, MaxLon: -178, MaxLat: -16}, 2, false},
		{"the pole clamps", domain.BBox{MinLon: 0, MinLat: 88, MaxLon: 1, MaxLat: 90}, 1, false},
		{"a continent", domain.BBox{MinLon: -10, MinLat: 35, MaxLon: 30, MaxLat: 60}, 0, true},
		{"six cells", domain.BBox{MinLon: -9.9, MinLat: 36, MaxLon: 4.9, MaxLat: 44}, 0, true},
	}
	for _, tc := range cases {
		got, err := cells(tc.bb, 4)
		if tc.err {
			if !errors.Is(err, ErrTooLarge) {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err != nil || len(got) != tc.want {
			t.Errorf("%s: %d cells, %v; want %d", tc.name, len(got), err, tc.want)
		}
	}
}

func TestInBoxCachesDedupesAndFilters(t *testing.T) {
	s, f, c := setup(Options{},
		// On the 0° meridian between two cells: both circles hold it.
		plane("shared", -5, 40, t0),
		plane("lisbon", -9.1, 38.7, t0),
		plane("outside", -9.95, 38.7, t0), // in a fetched cell, outside the bbox
	)
	ctx := context.Background()
	res, err := s.InBox(ctx, iberia)
	if err != nil || !res.Complete || !res.DataAt.Equal(t0) {
		t.Fatalf("%+v %v", res, err)
	}
	if len(res.Aircraft) != 2 || res.Aircraft[0].Hex != "lisbon" || res.Aircraft[1].Hex != "shared" {
		t.Fatalf("aircraft %+v", res.Aircraft)
	}
	if n := f.calls.Load(); n != 4 {
		t.Fatalf("%d feed calls, want one per cell", n)
	}

	c.add(30 * time.Second)
	if _, err := s.InBox(ctx, iberia); err != nil || f.calls.Load() != 4 {
		t.Fatalf("within the TTL: %d calls, %v", f.calls.Load(), err)
	}
	c.add(31 * time.Second)
	res, _ = s.InBox(ctx, iberia)
	if f.calls.Load() != 8 || !res.DataAt.Equal(t0.Add(61*time.Second)) {
		t.Fatalf("after the TTL: %d calls, data at %s", f.calls.Load(), res.DataAt)
	}
}

func TestOneRequestPerCellInFlight(t *testing.T) {
	s, f, _ := setup(Options{}, plane("a", -9.1, 38.7, t0))
	f.gate = make(chan struct{})
	bb := domain.BBox{MinLon: -9.3, MinLat: 38.6, MaxLon: -9.0, MaxLat: 38.8}
	var wg sync.WaitGroup
	results := make([]Result, 8)
	for i := range results {
		wg.Go(func() { results[i], _ = s.InBox(context.Background(), bb) })
	}
	for f.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // let the others queue on the request
	close(f.gate)
	wg.Wait()
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("%d feed calls for one cell, want 1", n)
	}
	for _, r := range results {
		if len(r.Aircraft) != 1 {
			t.Fatalf("a waiter got %+v", r)
		}
	}
}

func TestQuotaServesStaleThenNothing(t *testing.T) {
	s, f, c := setup(Options{}, plane("a", -9.1, 38.7, t0))
	ctx := context.Background()
	bb := domain.BBox{MinLon: -9.3, MinLat: 38.6, MaxLon: -9.0, MaxLat: 38.8}
	if _, err := s.InBox(ctx, bb); err != nil {
		t.Fatal(err)
	}

	f.fail(&domain.QuotaError{RetryAfter: 5 * time.Minute})
	c.add(2 * time.Minute)
	res, _ := s.InBox(ctx, bb) // asks once, gets 429, serves stale
	if !res.Complete || !res.DataAt.Equal(t0) || len(res.Aircraft) != 1 || f.calls.Load() != 2 {
		t.Fatalf("stale: %+v after %d calls", res, f.calls.Load())
	}
	c.add(4 * time.Minute) // still inside Retry-After
	if _, _ = s.InBox(ctx, bb); f.calls.Load() != 2 {
		t.Fatalf("asked during Retry-After: %d calls", f.calls.Load())
	}
	f.fail(nil)
	c.add(5 * time.Minute) // past Retry-After: asking is allowed again
	if res, _ := s.InBox(ctx, bb); f.calls.Load() != 3 || !res.Complete {
		t.Fatalf("after Retry-After: %d calls, %+v", f.calls.Load(), res)
	}

	f.fail(&domain.QuotaError{})
	c.add(16 * time.Minute)
	res, _ = s.InBox(ctx, bb)
	if res.Complete || len(res.Aircraft) != 0 || !res.DataAt.IsZero() {
		t.Fatalf("past MaxStale: %+v", res)
	}
}

func TestReserveAndErrorsPause(t *testing.T) {
	s, f, c := setup(Options{Reserve: 50}, plane("a", -9.1, 38.7, t0))
	ctx := context.Background()
	one := domain.BBox{MinLon: -9.3, MinLat: 38.6, MaxLon: -9.0, MaxLat: 38.8}
	other := domain.BBox{MinLon: 2.2, MinLat: 48.8, MaxLon: 2.4, MaxLat: 48.9}

	f.remaining = 50
	if _, err := s.InBox(ctx, one); err != nil {
		t.Fatal(err)
	}
	res, _ := s.InBox(ctx, other) // the feed said 50 left: paused
	if res.Complete || f.calls.Load() != 1 {
		t.Fatalf("at the reserve: %+v, %d calls", res, f.calls.Load())
	}
	c.add(31 * time.Minute)
	f.remaining = 400
	f.fail(errors.New("HTTP 502"))
	if _, _ = s.InBox(ctx, other); f.calls.Load() != 2 {
		t.Fatalf("after the pause: %d calls", f.calls.Load())
	}
	c.add(10 * time.Second)
	if _, _ = s.InBox(ctx, other); f.calls.Load() != 2 {
		t.Fatalf("retried a failed cell inside the backoff: %d calls", f.calls.Load())
	}
}

func TestPaceSpreadsTheQuota(t *testing.T) {
	s, f, _ := setup(Options{PerDay: 24, Burst: 1}) // one an hour
	res, _ := s.InBox(context.Background(), iberia)
	if f.calls.Load() != 1 || res.Complete {
		t.Fatalf("burst 1 over 4 cells: %d calls, complete %v", f.calls.Load(), res.Complete)
	}
}

// TestSteadyState sizes the defaults to the keyless quota: after the
// burst, one viewer polling every minute gets complete answers, a 4-cell
// view up to 12 minutes old and a single cell up to 3.
func TestSteadyState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bb    domain.BBox
		worst time.Duration
	}{
		{"one cell", domain.BBox{MinLon: -9.9, MinLat: 36, MaxLon: -5.1, MaxLat: 39}, 3 * time.Minute},
		{"four cells", iberia, 12 * time.Minute},
	} {
		s, _, c := setup(Options{})
		var worst time.Duration
		for m := range 180 {
			res, err := s.InBox(context.Background(), tc.bb)
			if err != nil {
				t.Fatal(err)
			}
			if m >= 30 { // past the burst
				if !res.Complete {
					t.Fatalf("%s: incomplete at minute %d", tc.name, m)
				}
				worst = max(worst, c.now().Sub(res.DataAt))
			}
			c.add(time.Minute)
		}
		if worst > tc.worst {
			t.Errorf("%s: data up to %s old, want at most %s", tc.name, worst, tc.worst)
		}
	}
}

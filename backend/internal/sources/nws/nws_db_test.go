package nws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/store"
	"shield/internal/testutil"
)

// Against PostGIS: an unchanged list writes nothing (no revision, no
// stream change); a segment canceled shrinks the event; the last one
// canceled ends it.
func TestPollStoresAndEnds(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	square := func(x, y float64) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"type": "MultiLineString", "coordinates": [][][2]float64{
			{{x, y}, {x, y + 0.5}, {x + 0.5, y + 0.5}, {x + 0.5, y}, {x, y}}, // clockwise: an outer ring
		}})
		return b
	}
	zones := store.NewNWSZoneRepository(pool)
	if err := zones.Load(ctx, "forecast", []domain.ForecastZone{
		{UGC: "ORZ101", State: "OR", Name: "North Oregon Coast", Rings: square(-124, 45.5)},
		{UGC: "WAZ201", State: "WA", Name: "South Washington Coast", Rings: square(-124.2, 46.3)},
		{UGC: "FLZ038", State: "FL", Name: "Coastal Duval", Rings: square(-81.5, 30.2)},
		{UGC: "FLZ138", State: "FL", Name: "Coastal St. Johns", Rings: square(-81.4, 29.7)},
	}, 0, "nws_zones", "test"); err != nil {
		t.Fatal(err)
	}
	events := store.NewEventRepository(pool)
	s0 := New(nil, nil, nil, nil)
	if err := store.NewSourceRepository(pool).Register(ctx, s0.Info()); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	body := string(fixture(t))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), events, zones, slog.New(slog.DiscardHandler))
	s.url = srv.URL
	s.now = func() time.Time { return time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC) }
	poll := func() int {
		t.Helper()
		res, err := s.Poll(ctx, domain.Validators{})
		if err != nil {
			t.Fatal(err)
		}
		return res.Upserted
	}
	var changes, revisions int
	count := func() (int, int) {
		t.Helper()
		var c, r int
		if err := pool.QueryRow(ctx, `SELECT COALESCE(max(change_seq), 0) FROM events`).Scan(&c); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_revisions`).Scan(&r); err != nil {
			t.Fatal(err)
		}
		return c, r
	}

	// The flash flood warning draws its polygon; the coastal flood
	// advisory and warning their zones. Nothing else is drawable here.
	if n := poll(); n != 3 {
		t.Fatalf("first poll stored %d", n)
	}
	changes, revisions = count()
	if n := poll(); n != 0 {
		t.Fatalf("an unchanged list changed %d events", n)
	}
	if c, r := count(); c != changes || r != revisions {
		t.Fatalf("an unchanged list moved the stream (%d -> %d) or wrote revisions (%d -> %d)", changes, c, revisions, r)
	}

	area := func() float64 {
		t.Helper()
		var a float64
		if err := pool.QueryRow(ctx, `SELECT ST_Area(geom::geometry) FROM events WHERE source_id = $1 AND source_event_id = 'KPQR.CF.Y.0005.2026'`, ID).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	both := area()
	// Washington's segment is canceled: the event shrinks to Oregon.
	mu.Lock()
	body = strings.Replace(body, "/O.CON.KPQR.CF.Y.0005.000000T0000Z-261128T0100Z/", "/O.CAN.KPQR.CF.Y.0005.000000T0000Z-261128T0100Z/", 1)
	mu.Unlock()
	if n := poll(); n != 1 {
		t.Fatalf("partial cancellation changed %d", n)
	}
	if a := area(); a >= both*0.6 {
		t.Fatalf("area %v of %v after one segment ended", a, both)
	}
	// Oregon's too: the event ends.
	mu.Lock()
	body = strings.Replace(body, "/O.CON.KPQR.CF.Y.0005.000000T0000Z-261127T0000Z/", "/O.CAN.KPQR.CF.Y.0005.000000T0000Z-261127T0000Z/", 1)
	mu.Unlock()
	if n := poll(); n != 1 {
		t.Fatalf("cancellation changed %d", n)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM events WHERE source_id = $1 AND source_event_id = 'KPQR.CF.Y.0005.2026'`, ID).Scan(&status); err != nil || status != "resolved" {
		t.Fatalf("status %q %v", status, err)
	}
}

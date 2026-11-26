package emsc

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/sources/usgs"
	"shield/internal/store"
	"shield/internal/testutil"
)

// Against PostGIS, on the week recorded on 2026-11-26 from both services:
// EMSC is stored first (it is faster in Europe); once USGS lists its
// quakes, every EMSC copy of one goes, and only the three USGS never
// listed stay among those of M3.8 and up: Iceland 4.0, Algeria 4.0, and
// the Turkey–Iran 3.9 aftershock 17 minutes after a 4.5 USGS has. They
// stay out once USGS ends its own.
func TestPollAgainstUSGS(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	sources := store.NewSourceRepository(pool)
	for _, info := range []domain.SourceInfo{New(nil, nil, nil).Info(), usgs.New(nil, nil).Info()} {
		if err := sources.Register(ctx, info); err != nil {
			t.Fatal(err)
		}
	}
	events := store.NewEventRepository(pool)
	body := read(t, "emsc-week-m3.json")
	s, _ := newSource(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }, events)

	poll := func() {
		t.Helper()
		if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
			t.Fatal(err)
		}
	}
	stored := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT source_event_id FROM events
			WHERE source_id = $1 AND (attrs ->> 'mag')::float8 >= 3.8 ORDER BY 1`, ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		return ids
	}
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE source_id = $1`, ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	poll()
	if n := count(); n != 51 {
		t.Fatalf("before USGS: %d stored, want all 51", n)
	}
	u, err := usgs.Parse(read(t, "usgs-week-euromed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := events.Upsert(ctx, usgs.ID, u.Events); err != nil {
		t.Fatal(err)
	}
	poll()
	want := []string{"20261121_0000279", "20261125_0000105", "20261125_0000288"}
	if got := stored(); !slices.Equal(got, want) {
		t.Fatalf("M3.8+ kept %v, want %v", got, want)
	}
	if n := count(); n != 51-len(u.Events) {
		t.Fatalf("%d stored, want 51 less USGS's %d", n, len(u.Events))
	}

	ids := make([]string, len(u.Events))
	for i := range u.Events {
		ids[i] = u.Events[i].SourceEventID
	}
	if _, err := events.Resolve(ctx, usgs.ID, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
	poll()
	if got := stored(); !slices.Equal(got, want) {
		t.Fatalf("after USGS ended them, M3.8+ %v, want %v", got, want)
	}
	// Each copy removed left a tombstone, so clients drop it.
	var tombs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_tombstones`).Scan(&tombs); err != nil || tombs != len(u.Events) {
		t.Fatalf("%d tombstones, want %d, %v", tombs, len(u.Events), err)
	}
	// USGS retracts them: EMSC's come back.
	if _, err := events.Delete(ctx, usgs.ID, ids); err != nil {
		t.Fatal(err)
	}
	poll()
	if n := count(); n != 51 {
		t.Fatalf("after USGS deleted its own, %d stored, want 51", n)
	}
}

// USGS stored first (a deploy, a restart): every EMSC quake is unstored,
// so the looser match decides, and it keeps the same three as the strict
// one on the recorded week.
func TestPollAfterUSGS(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	sources := store.NewSourceRepository(pool)
	for _, info := range []domain.SourceInfo{New(nil, nil, nil).Info(), usgs.New(nil, nil).Info()} {
		if err := sources.Register(ctx, info); err != nil {
			t.Fatal(err)
		}
	}
	events := store.NewEventRepository(pool)
	u, err := usgs.Parse(read(t, "usgs-week-euromed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := events.Upsert(ctx, usgs.ID, u.Events); err != nil {
		t.Fatal(err)
	}
	body := read(t, "emsc-week-m3.json")
	s, _ := newSource(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }, events)
	if _, err := s.Poll(ctx, domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	var n int
	var big []string
	if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(array_agg(source_event_id ORDER BY source_event_id)
			FILTER (WHERE (attrs ->> 'mag')::float8 >= 3.8), '{}') FROM events WHERE source_id = $1`, ID).Scan(&n, &big); err != nil {
		t.Fatal(err)
	}
	want := []string{"20261121_0000279", "20261125_0000105", "20261125_0000288"}
	if n != 51-len(u.Events) || !slices.Equal(big, want) {
		t.Fatalf("%d stored, M3.8+ %v; want %d and %v", n, big, 51-len(u.Events), want)
	}
}

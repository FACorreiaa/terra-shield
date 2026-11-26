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
	for _, info := range []domain.SourceInfo{New(nil, nil).Info(), usgs.New(nil, nil).Info()} {
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
}

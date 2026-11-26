package store_test

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

// Each way an event can change over a window, against PostGIS.
func TestChanges(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	ctx := context.Background()
	events := store.NewEventRepository(pool)
	changes := store.NewChangeRepository(pool)

	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	square := func(x float64) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][2]float64{
			{{x, 38}, {x, 39}, {x + 0.5, 39}, {x + 0.5, 38}, {x, 38}},
		}})
		return b
	}
	ev := func(id string, typ domain.EventType, sev string, status domain.Status) domain.Event {
		return domain.Event{
			SourceEventID: id, Type: typ, Kind: domain.KindWarning, Status: status, Severity: sev,
			Title: id, Geometry: square(-8), ObservedAt: at, UpdatedAt: at,
		}
	}
	upsert := func(es ...domain.Event) {
		t.Helper()
		if _, err := events.Upsert(ctx, "test", es); err != nil {
			t.Fatal(err)
		}
	}
	// before moves an event's revisions so far written to two days ago,
	// before the window.
	before := func(id string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE event_revisions SET at = now() - interval '2 days'
			WHERE event_id = (SELECT id FROM events WHERE source_event_id = $1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	// age makes an event look first seen two days ago, before the window.
	age := func(ids ...string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE events SET first_seen_at = now() - interval '2 days' WHERE source_event_id = ANY($1)`, ids); err != nil {
			t.Fatal(err)
		}
	}

	// Stored before first-seen times existed, then nothing: not new.
	upsert(ev("legacy", domain.TypeFlood, "moderate", domain.StatusActive))
	if _, err := pool.Exec(ctx, `UPDATE events SET first_seen_at = NULL WHERE source_event_id = 'legacy'`); err != nil {
		t.Fatal(err)
	}
	// Arrived in the window.
	upsert(ev("arrived", domain.TypeFlood, "minor", domain.StatusActive))
	// Arrived already resolved: never active in Shield.
	upsert(ev("born-resolved", domain.TypeFlood, "severe", domain.StatusResolved))
	// Arrived and ended inside the window: ended, once.
	upsert(ev("brief", domain.TypeWind, "moderate", domain.StatusActive))
	if _, err := events.Resolve(ctx, "test", []string{"brief"}, at); err != nil {
		t.Fatal(err)
	}
	// Older events that changed in the window.
	upsert(
		ev("rising", domain.TypeFlood, "minor", domain.StatusActive),
		ev("falling", domain.TypeFlood, "extreme", domain.StatusActive),
		ev("round-trip", domain.TypeFlood, "minor", domain.StatusActive),
		ev("reshaped", domain.TypeFlood, "minor", domain.StatusActive),
		ev("back", domain.TypeFlood, "minor", domain.StatusActive),
		ev("flap", domain.TypeFlood, "minor", domain.StatusActive),
		ev("relapse", domain.TypeFlood, "minor", domain.StatusActive),
		ev("quake", domain.TypeEarthquake, "minor", domain.StatusActive),
		ev("war", domain.TypeConflict, "severe", domain.StatusActive),
	)
	age("rising", "falling", "round-trip", "reshaped", "back", "flap", "relapse", "quake", "war")
	rising := ev("rising", domain.TypeFlood, "moderate", domain.StatusActive)
	upsert(rising)
	rising.Severity = "severe"
	upsert(rising)
	falling := ev("falling", domain.TypeFlood, "moderate", domain.StatusActive)
	upsert(falling)
	trip := ev("round-trip", domain.TypeFlood, "severe", domain.StatusActive)
	upsert(trip)
	trip.Severity = "minor"
	upsert(trip)
	reshaped := ev("reshaped", domain.TypeFlood, "minor", domain.StatusActive)
	reshaped.Geometry = square(-7)
	upsert(reshaped)
	// Ended before the window, listed again in it: new.
	if _, err := events.Resolve(ctx, "test", []string{"back", "relapse"}, at); err != nil {
		t.Fatal(err)
	}
	before("back")
	before("relapse")
	upsert(ev("back", domain.TypeFlood, "minor", domain.StatusActive))
	// Ended before the window, listed again and ended again in it: ended
	// at both ends, no change.
	upsert(ev("relapse", domain.TypeFlood, "minor", domain.StatusActive))
	if _, err := events.Resolve(ctx, "test", []string{"relapse"}, at); err != nil {
		t.Fatal(err)
	}
	// Active before the window, ended by a feed blip and listed again
	// higher: not new, escalated.
	if _, err := events.Resolve(ctx, "test", []string{"flap"}, at); err != nil {
		t.Fatal(err)
	}
	upsert(ev("flap", domain.TypeFlood, "severe", domain.StatusActive))
	quake := ev("quake", domain.TypeEarthquake, "severe", domain.StatusActive)
	quake.Geometry = square(2) // Spain, not Portugal
	upsert(quake)
	upsert(ev("war", domain.TypeConflict, "extreme", domain.StatusActive))

	list := func(q domain.ChangeQuery) map[string]domain.Change {
		t.Helper()
		if q.Since.IsZero() {
			q.Since = time.Now().Add(-time.Hour)
		}
		if q.Types == nil {
			q.Types = domain.HazardTypes()
		}
		rows, err := changes.List(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]domain.Change{}
		for _, r := range rows {
			if _, dup := out[r.Title]; dup {
				t.Fatalf("%s listed twice", r.Title)
			}
			out[r.Title] = r
		}
		return out
	}
	got := list(domain.ChangeQuery{})
	want := map[string]domain.ChangeKind{
		"arrived": domain.ChangeNew,
		"brief":   domain.ChangeEnded,
		"rising":  domain.ChangeEscalated,
		"falling": domain.ChangeDowngraded,
		"back":    domain.ChangeNew,
		"flap":    domain.ChangeEscalated,
		"quake":   domain.ChangeEscalated,
	}
	for title, kind := range want {
		if got[title].Change != kind {
			t.Errorf("%s: %q, want %q", title, got[title].Change, kind)
		}
	}
	for _, title := range slices.Sorted(maps.Keys(got)) {
		if _, ok := want[title]; !ok {
			t.Errorf("%s listed as %q, want left out", title, got[title].Change)
		}
	}
	if r := got["rising"]; r.FromSeverity != "minor" || r.Severity != "severe" {
		t.Errorf("rising from %q to %q, want minor to severe", r.FromSeverity, r.Severity)
	}
	if r := got["flap"]; r.FromSeverity != "minor" {
		t.Errorf("flap from %q", r.FromSeverity)
	}
	if r := got["falling"]; r.FromSeverity != "extreme" {
		t.Errorf("falling from %q", r.FromSeverity)
	}
	if r := got["arrived"]; r.FromSeverity != "" || !slices.Equal(r.Countries, []string{"PT"}) || r.At.IsZero() {
		t.Errorf("arrived: %+v", r)
	}

	// Filters: country, type, severity; conflicts only when named.
	if got := list(domain.ChangeQuery{Country: "ES"}); len(got) != 1 || got["quake"].Change != domain.ChangeEscalated {
		t.Errorf("country ES: %v", slices.Collect(maps.Keys(got)))
	}
	if got := list(domain.ChangeQuery{Types: []domain.EventType{domain.TypeWind}}); len(got) != 1 || got["brief"].Change != domain.ChangeEnded {
		t.Errorf("type wind: %v", slices.Collect(maps.Keys(got)))
	}
	if got := list(domain.ChangeQuery{Severities: []string{"severe"}}); len(got) != 3 {
		t.Errorf("severity severe: %v", slices.Collect(maps.Keys(got)))
	}
	if got := list(domain.ChangeQuery{Types: []domain.EventType{domain.TypeConflict}}); got["war"].Change != domain.ChangeEscalated {
		t.Errorf("conflict when named: %v", got)
	}
	// A window after everything: nothing.
	if got := list(domain.ChangeQuery{Since: time.Now().Add(time.Minute)}); len(got) != 0 {
		t.Errorf("empty window: %v", slices.Collect(maps.Keys(got)))
	}
}

// A new version of a warning (MeteoAlarm Update, new id) carries on the
// old one's first-seen time, and a severity change across versions is an
// escalation, not a new warning.
func TestChangesAcrossVersions(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	ctx := context.Background()
	events := store.NewEventRepository(pool)
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	geom, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{-8, 39}})
	ev := func(id, sev string, supersedes ...string) domain.Event {
		return domain.Event{
			SourceEventID: id, Type: domain.TypeWind, Kind: domain.KindWarning, Status: domain.StatusActive, Severity: sev,
			Title: id, Geometry: geom, ObservedAt: at, UpdatedAt: at, Supersedes: supersedes,
		}
	}
	upsert := func(es ...domain.Event) {
		t.Helper()
		if _, err := events.Upsert(ctx, "test", es); err != nil {
			t.Fatal(err)
		}
	}
	// replace stores new versions, then deletes the ones they name, as
	// the source does.
	replace := func(es ...domain.Event) {
		t.Helper()
		upsert(es...)
		var gone []string
		for i := range es {
			gone = append(gone, es[i].Supersedes...)
		}
		if _, err := events.Delete(ctx, "test", gone); err != nil {
			t.Fatal(err)
		}
	}
	upsert(
		ev("v1", "moderate"), ev("w1", "moderate"), ev("x1", "moderate"), ev("y1", "moderate"),
		ev("c1", "moderate"), ev("d1", "moderate"), ev("r1", "moderate"),
		ev("m1a", "moderate"), ev("m1b", "severe"), ev("o1", "moderate"),
	)
	if _, err := pool.Exec(ctx, `UPDATE events SET first_seen_at = now() - interval '2 days'
		WHERE source_event_id IN ('v1', 'w1', 'c1', 'd1', 'r1', 'm1a', 'm1b', 'o1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE events SET first_seen_at = NULL WHERE source_event_id IN ('x1', 'y1')`); err != nil {
		t.Fatal(err)
	}
	// r1 ended (the expiry sweep) before its next version arrived; o1
	// ended before the window, so its next version is active again.
	if _, err := events.Resolve(ctx, "test", []string{"r1", "o1"}, at); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE event_revisions SET at = now() - interval '2 days'
		WHERE event_id = (SELECT id FROM events WHERE source_event_id = 'o1')`); err != nil {
		t.Fatal(err)
	}
	replace(
		ev("v2", "severe", "v1"),         // escalated
		ev("w2", "moderate", "w1"),       // same level: no change
		ev("x2", "moderate", "x1"),       // first seen unknown: never new
		ev("y2", "severe", "y1"),         // first seen unknown, escalated
		ev("c2", "severe", "c1"),         // c1 -> c2 -> c3, severe twice
		ev("d2", "severe", "d1"),         // d1 -> d2 -> d3, back to moderate
		ev("r2", "severe", "r1"),         // ended in the window: escalated
		ev("m2", "severe", "m1a", "m1b"), // the highest it replaced
		ev("u2", "minor", "not-stored"),  // nothing stored to carry on
		ev("o2", "moderate", "o1"),       // ended before the window: new
	)
	replace(ev("c3", "severe", "c2"), ev("d3", "moderate", "d2"))
	rows, err := store.NewChangeRepository(pool).List(ctx, domain.ChangeQuery{Since: time.Now().Add(-time.Hour), Types: domain.HazardTypes()})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.Title] = string(r.Change) + " " + r.FromSeverity
	}
	want := map[string]string{
		"v2": "escalated moderate",
		"y2": "escalated moderate",
		"c3": "escalated moderate",
		"r2": "escalated moderate",
		"u2": "new ",
		"o2": "new ",
	}
	if !maps.Equal(got, want) {
		t.Fatalf("changes %v\nwant %v", got, want)
	}
	// The chain keeps one history: c3 holds c2's revision and its own none.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_revisions r JOIN events e ON e.id = r.event_id
		WHERE e.source_event_id = 'c3'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("c3 has %d revisions, %v", n, err)
	}
}

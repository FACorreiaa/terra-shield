package store_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

var listNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// seedEvents stores six events around the world (see seedWorld for PT/ES).
func seedEvents(t *testing.T, pool *pgxpool.Pool) *store.EventRepository {
	t.Helper()
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	mk := func(id string, typ domain.EventType, sev string, lon, lat float64, age time.Duration) domain.Event {
		e := quake(id, lon, lat, 5, listNow.Add(-age))
		e.Type, e.Severity = typ, sev
		return e
	}
	resolved := mk("resolved-pt", domain.TypeFlood, "extreme", -8, 40, time.Hour)
	ended := listNow.Add(-30 * time.Minute)
	resolved.Status, resolved.EndedAt = domain.StatusResolved, &ended
	square, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][]float64{
		{{-5, 40}, {-4, 40}, {-4, 41}, {-5, 41}, {-5, 40}},
	}})
	drought := mk("drought-es", domain.TypeDrought, "severe", 0, 0, 2*time.Hour)
	drought.Geometry = square
	drought.Attrs = map[string]any{"severityProvisional": true, "n": 1.5}

	events := []domain.Event{
		mk("quake-pt", domain.TypeEarthquake, "moderate", -8, 39, time.Hour),
		mk("quake-off-pt", domain.TypeEarthquake, "minor", -10.5, 38.5, 3*time.Hour), // offshore: nearby PT
		drought,
		mk("fiji-east", domain.TypeTropicalCyclone, "extreme", 179.5, -17, 5*time.Hour),
		mk("fiji-west", domain.TypeTropicalCyclone, "minor", -179.5, -17, 4*time.Hour),
		resolved,
	}
	if _, err := repo.Upsert(context.Background(), "test", events); err != nil {
		t.Fatal(err)
	}
	return repo
}

func ids(recs []domain.EventRecord) []string {
	out := make([]string, len(recs))
	for i := range recs {
		out[i] = recs[i].SourceEventID
	}
	return out
}

func TestEventListFiltersAndOrder(t *testing.T) {
	pool := testutil.DB(t)
	repo := seedEvents(t, pool)
	ctx := context.Background()
	since := listNow.Add(-150 * time.Minute)
	endedWindow, afterEnd := listNow.Add(-45*time.Minute), listNow.Add(-20*time.Minute)
	resolved := []domain.Status{domain.StatusResolved}

	cases := []struct {
		name string
		q    domain.EventQuery
		want []string
	}{
		{
			"active, severity then recency",
			domain.EventQuery{Statuses: []domain.Status{domain.StatusActive}},
			[]string{"fiji-east", "drought-es", "quake-pt", "quake-off-pt", "fiji-west"},
		},
		{
			"any status",
			domain.EventQuery{},
			[]string{"resolved-pt", "fiji-east", "drought-es", "quake-pt", "quake-off-pt", "fiji-west"},
		},
		{
			"resolved only",
			domain.EventQuery{Statuses: []domain.Status{domain.StatusResolved}},
			[]string{"resolved-pt"},
		},
		{
			"active and resolved",
			domain.EventQuery{Statuses: []domain.Status{domain.StatusActive, domain.StatusResolved}},
			[]string{"resolved-pt", "fiji-east", "drought-es", "quake-pt", "quake-off-pt", "fiji-west"},
		},
		{
			"types",
			domain.EventQuery{Types: []domain.EventType{domain.TypeDrought, domain.TypeFlood}},
			[]string{"resolved-pt", "drought-es"},
		},
		{"severities", domain.EventQuery{Severities: []string{"minor"}}, []string{"quake-off-pt", "fiji-west"}},
		{"kinds", domain.EventQuery{Kinds: []domain.Kind{domain.KindWarning}}, nil},
		{"sources", domain.EventQuery{Sources: []string{"nope"}}, nil},
		{
			"country includes nearby",
			domain.EventQuery{Country: "PT", Statuses: []domain.Status{domain.StatusActive}},
			[]string{"quake-pt", "quake-off-pt"},
		},
		{
			"since/until on updatedAt",
			domain.EventQuery{Since: &since, Until: &listNow},
			[]string{"resolved-pt", "drought-es", "quake-pt"},
		},
		{
			// resolved-pt was last updated 1 h ago and ended 30 min ago: a
			// resolved event is in the window by its end.
			"resolved: window on endedAt",
			domain.EventQuery{Statuses: resolved, Since: &endedWindow},
			[]string{"resolved-pt"},
		},
		{"resolved: ended before the window", domain.EventQuery{Statuses: resolved, Since: &afterEnd}, nil},
		{"resolved: until before the end", domain.EventQuery{Statuses: resolved, Until: &endedWindow}, nil},
		{
			"bbox",
			domain.EventQuery{BBox: &domain.BBox{MinLon: -9, MinLat: 38, MaxLon: -3.5, MaxLat: 42}},
			[]string{"resolved-pt", "drought-es", "quake-pt"},
		},
		{
			"bbox across the antimeridian",
			domain.EventQuery{BBox: &domain.BBox{MinLon: 179, MinLat: -20, MaxLon: -179, MaxLat: -10}},
			[]string{"fiji-east", "fiji-west"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.q.Limit = 100
			got, truncated, err := repo.List(ctx, c.q)
			if err != nil {
				t.Fatal(err)
			}
			if truncated || !slices.Equal(ids(got), c.want) {
				t.Fatalf("got %v truncated=%v, want %v", ids(got), truncated, c.want)
			}
		})
	}
}

func TestEventListRecord(t *testing.T) {
	pool := testutil.DB(t)
	repo := seedEvents(t, pool)
	ctx := context.Background()

	got, truncated, err := repo.List(ctx, domain.EventQuery{Limit: 2})
	if err != nil || !truncated || len(got) != 2 {
		t.Fatalf("limit: %d truncated=%v err %v", len(got), truncated, err)
	}

	recs, _, err := repo.List(ctx, domain.EventQuery{Limit: 10, Country: "PT", Severities: []string{"minor"}})
	if err != nil || len(recs) != 1 {
		t.Fatalf("%v %v", ids(recs), err)
	}
	off := recs[0]
	if len(off.Countries) != 1 || off.Countries[0].Relation != domain.RelationNearby || off.Countries[0].DistanceKm <= 0 {
		t.Fatalf("offshore countries %+v", off.Countries)
	}
	if off.ID == "" || off.SourceID != "test" || off.Latitude != 38.5 || off.Longitude != -10.5 || off.FetchedAt.IsZero() ||
		off.Geometry != nil || off.Attrs["mag"] != 5.0 {
		t.Fatalf("record %+v", off)
	}

	recs, _, err = repo.List(ctx, domain.EventQuery{Limit: 10, Types: []domain.EventType{domain.TypeDrought}, Geometry: true, SimplifyDegrees: 0.1})
	if err != nil || len(recs) != 1 {
		t.Fatalf("%v %v", ids(recs), err)
	}
	d := recs[0]
	var g struct{ Type string }
	if err := json.Unmarshal(d.Geometry, &g); err != nil || g.Type != "Polygon" {
		t.Fatalf("geometry %s err %v", d.Geometry, err)
	}
	if d.Attrs["severityProvisional"] != true || len(d.Countries) != 1 || d.Countries[0].ISO2 != "ES" {
		t.Fatalf("drought %+v", d)
	}

	if _, _, err := repo.List(ctx, domain.EventQuery{}); err == nil {
		t.Fatal("zero limit accepted")
	}
}

// near filters on geography: distances are on the ellipsoid, the order is
// nearest first whatever the severity, the antimeridian is no edge, and a
// point inside an area is 0 km from it.
func TestEventListNear(t *testing.T) {
	pool := testutil.DB(t)
	repo := seedEvents(t, pool)
	ctx := context.Background()
	active := []domain.Status{domain.StatusActive}

	cases := []struct {
		name string
		near domain.Near
		want []string
		km   []float64 // approximate, ±2 km
	}{
		{"nearest first, not most severe", domain.Near{Lon: -10.4, Lat: 38.5, RadiusKm: 250}, []string{"quake-off-pt", "quake-pt"}, []float64{8.7, 214}},
		{"radius excludes", domain.Near{Lon: -10.4, Lat: 38.5, RadiusKm: 50}, []string{"quake-off-pt"}, []float64{8.7}},
		{"across the antimeridian", domain.Near{Lon: 180, Lat: -17, RadiusKm: 100}, []string{"fiji-east", "fiji-west"}, []float64{53, 53}},
		{"inside an area", domain.Near{Lon: -4.5, Lat: 40.5, RadiusKm: 10}, []string{"drought-es"}, []float64{0}},
	}
	for _, c := range cases {
		recs, _, err := repo.List(ctx, domain.EventQuery{Statuses: active, Near: &c.near, Limit: 10})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := ids(recs); !slices.Equal(got, c.want) && !(c.name == "across the antimeridian" && slices.Equal(got, []string{"fiji-west", "fiji-east"})) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range recs {
			d := recs[i].DistanceKm
			if d == nil || *d < c.km[i]-2 || *d > c.km[i]+2 {
				t.Errorf("%s: %s distance %v, want about %v", c.name, recs[i].SourceEventID, d, c.km[i])
			}
		}
	}

	// Without near there is no distance.
	recs, _, err := repo.List(ctx, domain.EventQuery{Statuses: active, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for i := range recs {
		if recs[i].DistanceKm != nil {
			t.Errorf("%s has a distance without near", recs[i].SourceEventID)
		}
	}
}

// At a map zoom, a shape loses the parts too small to see, but never
// itself: an event of specks keeps its largest, a small storm survives
// zoom 2, and lines pass as they are.
func TestEventListMapGeometry(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	square := func(x, y, side float64) [][][2]float64 {
		return [][][2]float64{{{x, y}, {x + side, y}, {x + side, y + side}, {x, y + side}, {x, y}}}
	}
	shape := func(typ string, coords any) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"type": typ, "coordinates": coords})
		return b
	}
	event := func(id string, geom json.RawMessage) domain.Event {
		return domain.Event{
			SourceEventID: id, Type: domain.TypeFlood, Kind: domain.KindWarning, Status: domain.StatusActive,
			Severity: "moderate", Title: id, Geometry: geom, ObservedAt: listNow, UpdatedAt: listNow,
		}
	}
	if _, err := repo.Upsert(ctx, "test", []domain.Event{
		// One island of a degree and three of a few hundred metres.
		event("islands", shape("MultiPolygon", [][][][2]float64{
			square(-8.12345, 38.12345, 1), square(-6.5, 38, 0.003), square(-6.5, 38.5, 0.003), square(-6.5, 39, 0.003),
		})),
		// Every part below a pixel at zoom 2: the largest stays.
		event("specks", shape("MultiPolygon", [][][][2]float64{
			square(-5, 40, 0.01), square(-4.9, 40, 0.03), square(-4.8, 40, 0.01),
		})),
		event("storm", shape("Polygon", square(-3, 41, 0.05))),
		event("track", shape("LineString", [][2]float64{{-2, 42}, {-1.5, 42.001}, {-1, 42}})),
	}); err != nil {
		t.Fatal(err)
	}
	recs, _, err := repo.List(ctx, domain.EventQuery{Limit: 10, Geometry: true, SimplifyDegrees: 0.1758}) // zoom 2
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]struct {
		Type        string
		Coordinates json.RawMessage
	}{}
	for _, r := range recs {
		var g struct {
			Type        string
			Coordinates json.RawMessage
		}
		if err := json.Unmarshal(r.Geometry, &g); err != nil {
			t.Fatalf("%s: %s %v", r.Title, r.Geometry, err)
		}
		got[r.Title] = g
	}
	var parts [][][][2]float64
	if g := got["islands"]; g.Type != "MultiPolygon" || json.Unmarshal(g.Coordinates, &parts) != nil || len(parts) != 1 {
		t.Fatalf("islands %s %s", g.Type, g.Coordinates)
	}
	// Two decimals at zoom 2: -8.12345 is sent as -8.12.
	if c := parts[0][0][0]; c[0] != -8.12 || c[1] != 38.12 {
		t.Errorf("precision %v", c)
	}
	var ring [][][2]float64
	if g := got["specks"]; g.Type != "Polygon" || json.Unmarshal(g.Coordinates, &ring) != nil || len(ring) != 1 || ring[0][1][0]-ring[0][0][0] < 0.02 {
		t.Fatalf("specks %s %s", g.Type, g.Coordinates)
	}
	if g := got["storm"]; g.Type != "Polygon" || len(g.Coordinates) < 10 {
		t.Fatalf("storm %s %s", g.Type, g.Coordinates)
	}
	if g := got["track"]; g.Type != "LineString" {
		t.Fatalf("track %s %s", g.Type, g.Coordinates)
	}
}

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

	cases := []struct {
		name string
		q    domain.EventQuery
		want []string
	}{
		{
			"active, severity then recency",
			domain.EventQuery{Status: domain.StatusActive},
			[]string{"fiji-east", "drought-es", "quake-pt", "quake-off-pt", "fiji-west"},
		},
		{
			"any status",
			domain.EventQuery{},
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
			domain.EventQuery{Country: "PT", Status: domain.StatusActive},
			[]string{"quake-pt", "quake-off-pt"},
		},
		{
			"since/until on updatedAt",
			domain.EventQuery{Since: &since, Until: &listNow},
			[]string{"resolved-pt", "drought-es", "quake-pt"},
		},
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

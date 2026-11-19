package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestNWSZonesLoadAndUnion(t *testing.T) {
	pool := testutil.DB(t)
	repo := store.NewNWSZoneRepository(pool)
	ctx := context.Background()
	// Shapefile rings: outer clockwise, holes counter-clockwise.
	ring := func(x, y, s float64) [][2]float64 {
		return [][2]float64{{x, y}, {x, y + s}, {x + s, y + s}, {x + s, y}, {x, y}}
	}
	hole := func(x, y, s float64) [][2]float64 {
		return [][2]float64{{x, y}, {x + s, y}, {x + s, y + s}, {x, y + s}, {x, y}}
	}
	rings := func(rs ...[][2]float64) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"type": "MultiLineString", "coordinates": rs})
		return b
	}
	zones := []domain.ForecastZone{
		// An island with a lagoon: the counter-clockwise ring is a hole.
		{UGC: "GUZ001", State: "GU", Name: "Guam", Rings: rings(ring(144.6, 13.2, 0.4), hole(144.7, 13.3, 0.1))},
		{UGC: "HIZ001", State: "HI", Name: "Niihau", Rings: rings(ring(-160.3, 21.7, 0.3))},
	}
	if err := repo.Load(ctx, zones, 0.001, "nws_zones", "v1"); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Count(ctx); err != nil || n != 2 {
		t.Fatalf("count %d err %v", n, err)
	}
	var holes int
	if err := pool.QueryRow(ctx, `SELECT ST_NumInteriorRings(ST_GeometryN(geom, 1)) FROM nws_zones WHERE ugc = 'GUZ001'`).Scan(&holes); err != nil || holes != 1 {
		t.Fatalf("guam holes %d err %v", holes, err)
	}

	geom, found, err := repo.Union(ctx, []string{"HIZ001", "GUZ001", "XXZ999"})
	if err != nil || len(found) != 2 || found[0] != "GUZ001" || found[1] != "HIZ001" {
		t.Fatalf("found %v err %v", found, err)
	}
	var g struct{ Type string }
	if err := json.Unmarshal(geom, &g); err != nil || g.Type != "MultiPolygon" {
		t.Fatalf("geometry %s", geom)
	}
	if geom, found, err := repo.Union(ctx, []string{"XXZ999"}); err != nil || geom != nil || found != nil {
		t.Fatalf("unknown zones: %s %v %v", geom, found, err)
	}
	// Reloading replaces the set.
	if err := repo.Load(ctx, zones[1:], 0.001, "nws_zones", "v2"); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.Count(ctx); n != 1 {
		t.Fatalf("after reload %d", n)
	}
}

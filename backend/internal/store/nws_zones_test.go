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
	if err := repo.Load(ctx, "forecast", zones, 0.001, "nws_zones", "v1"); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Count(ctx, "forecast"); err != nil || n != 2 {
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
	// The fire set reuses a forecast code for another shape, and a county
	// set holds county codes: each is its own set, and Union reads only
	// forecast zones (tsunami warnings).
	if err := repo.Load(ctx, "fire", []domain.ForecastZone{
		{UGC: "HIZ001", State: "HI", Name: "Fire One", Rings: rings(ring(-155, 19, 0.5))},
	}, 0.001, "nws_fire_zones", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Load(ctx, "county", []domain.ForecastZone{
		{UGC: "HIC001", State: "HI", Name: "Hawaii County", Rings: rings(ring(-156, 18.9, 1.3))},
	}, 0.001, "nws_counties", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := repo.Union(ctx, []string{"HIZ001"}); len(found) != 1 {
		t.Fatalf("union reads forecast zones only: %v", found)
	}
	area := func(geom json.RawMessage) float64 {
		t.Helper()
		var a float64
		if err := pool.QueryRow(ctx, `SELECT ST_Area(ST_GeomFromGeoJSON($1))`, string(geom)).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	forecast, _, _ := repo.Draw(ctx, []domain.ZoneRef{{Set: "forecast", UGC: "HIZ001"}}, nil)
	fire, _, _ := repo.Draw(ctx, []domain.ZoneRef{{Set: "fire", UGC: "HIZ001"}}, nil)
	if forecast == nil || fire == nil || area(fire) < 2*area(forecast) {
		t.Fatalf("the fire zone is its own shape: %s / %s", forecast, fire)
	}
	// Zones of several sets and a polygon of the alert's own, in one area.
	poly := json.RawMessage(`{"type":"Polygon","coordinates":[[[-150,60],[-149,60],[-149,61],[-150,61],[-150,60]]]}`)
	refs := []domain.ZoneRef{{Set: "county", UGC: "HIC001"}, {Set: "forecast", UGC: "HIZ001"}, {Set: "county", UGC: "XXC999"}}
	geom, found, err = repo.Draw(ctx, refs, []json.RawMessage{poly})
	if err != nil || len(found) != 2 || found[0] != "county/HIC001" || found[1] != "forecast/HIZ001" {
		t.Fatalf("draw found %v err %v", found, err)
	}
	var mp struct {
		Type        string
		Coordinates [][][][2]float64
	}
	if err := json.Unmarshal(geom, &mp); err != nil || mp.Type != "MultiPolygon" || len(mp.Coordinates) != 3 {
		t.Fatalf("draw geometry %s", geom)
	}
	again, _, _ := repo.Draw(ctx, []domain.ZoneRef{refs[2], refs[1], refs[0]}, []json.RawMessage{poly})
	if string(again) != string(geom) {
		t.Fatal("the same area in another order draws differently")
	}
	if geom, found, err := repo.Draw(ctx, []domain.ZoneRef{{Set: "fire", UGC: "XXZ999"}}, nil); err != nil || geom != nil || len(found) != 0 {
		t.Fatalf("nothing to draw: %s %v %v", geom, found, err)
	}

	// Reloading replaces only that set.
	if err := repo.Load(ctx, "forecast", zones[1:], 0.001, "nws_zones", "v2"); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.Count(ctx, "forecast"); n != 1 {
		t.Fatalf("after reload %d", n)
	}
	if n, _ := repo.Count(ctx, "fire"); n != 1 {
		t.Fatalf("fire zones after a forecast reload: %d", n)
	}
}

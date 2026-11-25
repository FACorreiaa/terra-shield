package store_test

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestBasesReplaceAndQuery(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO countries (iso2, iso3, name, geom) VALUES
		('PT', 'PRT', 'Portugal', ST_Multi('POLYGON((-9.5 37,-6.2 37,-6.2 42.1,-9.5 42.1,-9.5 37))'::geometry)::geography)`); err != nil {
		t.Fatal(err)
	}
	repo := store.NewBaseRepository(pool)
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	bases := []domain.MilitaryBase{
		{QID: "Q1", Name: "Inland", Kind: domain.BaseAir, Class: "air base", Lon: -8, Lat: 39, Operator: "Air Force"},
		{QID: "Q2", Name: "Lajes", Kind: domain.BaseAir, Class: "air base", Lon: -27.1, Lat: 38.76}, // Azores: outside this test's PT polygon
		{QID: "Q3", Name: "Pacific", Kind: domain.BaseNaval, Class: "naval base", Lon: 179.5, Lat: -17},
	}
	n, err := repo.Replace(ctx, bases, map[string]string{"Q1": "ES", "Q2": "PT"}, at)
	if err != nil || n != 3 {
		t.Fatalf("replace %d %v", n, err)
	}
	all, err := repo.InBox(ctx, nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("all %d %v", len(all), err)
	}
	if all[0].ISO2 != "PT" || all[0].Operator != "Air Force" || !all[0].UpdatedAt.Equal(at) {
		t.Errorf("geometry beats the hint: %+v", all[0])
	}
	if all[1].ISO2 != "PT" || all[1].Description != "" {
		t.Errorf("hint fills in outside the geometry: %+v", all[1])
	}
	if all[2].ISO2 != "" {
		t.Errorf("no geometry, no hint: %+v", all[2])
	}
	across, err := repo.InBox(ctx, &domain.BBox{MinLon: 170, MinLat: -20, MaxLon: -170, MaxLat: -10})
	if err != nil || len(across) != 1 || across[0].QID != "Q3" {
		t.Fatalf("antimeridian %+v %v", across, err)
	}

	// Replace is wholesale.
	if _, err := repo.Replace(ctx, bases[:1], nil, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if c, err := repo.Count(ctx); err != nil || c != 1 {
		t.Fatalf("count %d %v", c, err)
	}
}

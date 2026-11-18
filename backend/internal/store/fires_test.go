package store_test

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestFireInsertDedupesAndAssignsCountry(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO countries (iso2, iso3, name, geom) VALUES
		('PT', 'PRT', 'Portugal', ST_Multi('POLYGON((-9.5 37,-6.2 37,-6.2 42.1,-9.5 42.1,-9.5 37))'::geometry)::geography)`); err != nil {
		t.Fatal(err)
	}
	repo := store.NewFireRepository(pool)
	t0 := time.Date(2026, 11, 16, 3, 12, 0, 0, time.UTC)
	frp := float32(4.2)
	inland := domain.FireDetection{Satellite: "NOAA-20", AcquiredAt: t0, Lat: 39.5, Lon: -8, FRP: &frp, Confidence: "nominal"}
	ocean := domain.FireDetection{Satellite: "NOAA-20", AcquiredAt: t0, Lat: 30, Lon: -30}

	// The file repeats a row within itself: stored once.
	n, err := repo.Insert(ctx, []domain.FireDetection{inland, ocean, inland})
	if err != nil || n != 2 {
		t.Fatalf("first insert %d err %v", n, err)
	}
	// The next poll repeats both: nothing new.
	later := inland
	later.AcquiredAt = t0.Add(time.Hour)
	if n, err = repo.Insert(ctx, []domain.FireDetection{inland, ocean, later}); err != nil || n != 1 {
		t.Fatalf("second insert %d err %v", n, err)
	}

	var iso2 *string
	if err := pool.QueryRow(ctx, `SELECT iso2 FROM fire_detections WHERE lat = 39.5 LIMIT 1`).Scan(&iso2); err != nil || iso2 == nil || *iso2 != "PT" {
		t.Fatalf("inland iso2 %v err %v", iso2, err)
	}
	if err := pool.QueryRow(ctx, `SELECT iso2 FROM fire_detections WHERE lat = 30`).Scan(&iso2); err != nil || iso2 != nil {
		t.Fatalf("ocean iso2 %v err %v", iso2, err)
	}

	if n, err := repo.PurgeBefore(ctx, t0.Add(time.Minute)); err != nil || n != 2 {
		t.Fatalf("purged %d err %v", n, err)
	}
	if c, _ := repo.Count(ctx); c != 1 {
		t.Fatalf("%d left, want the later one", c)
	}
}

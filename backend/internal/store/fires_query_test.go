package store_test

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestFireGridAndPoints(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	repo := store.NewFireRepository(pool)
	t0 := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	f := func(lon, lat, frp float32, age time.Duration) domain.FireDetection {
		v := frp
		return domain.FireDetection{Satellite: "NOAA-20", AcquiredAt: t0.Add(-age), Lat: float64(lat), Lon: float64(lon), FRP: &v, Confidence: "high"}
	}
	if _, err := repo.Insert(ctx, []domain.FireDetection{
		f(10.1, 45.1, 3, 0), f(10.2, 45.2, 9, time.Hour), f(10.3, 45.3, 1, 2*time.Hour), // one 1° cell
		f(-70.5, -30.5, 5, 0),                         // another
		f(179.5, -17.5, 2, 0), f(-179.5, -17.5, 2, 0), // either side of the antimeridian
		f(11, 46, 7, 30*time.Hour), // old
	}); err != nil {
		t.Fatal(err)
	}
	since := t0.Add(-24 * time.Hour)

	cells, err := repo.Grid(ctx, domain.FireQuery{Since: since}, 1)
	if err != nil || len(cells) != 4 {
		t.Fatalf("%+v %v", cells, err)
	}
	c := cells[0]
	if c.Count != 3 || c.Lat != 45.5 || c.Lon != 10.5 || *c.MaxFRP != 9 || !c.LatestAt.Equal(t0) {
		t.Fatalf("biggest cell %+v", c)
	}
	if all, _ := repo.Grid(ctx, domain.FireQuery{}, 1); len(all) != 5 {
		t.Fatalf("no since: %d cells", len(all))
	}

	pts, truncated, err := repo.Points(ctx, domain.FireQuery{Since: since, BBox: &domain.BBox{MinLon: 10, MinLat: 45, MaxLon: 10.25, MaxLat: 46}}, 10)
	if err != nil || truncated || len(pts) != 2 || pts[0].FRP == nil || *pts[0].FRP != 3 || pts[0].Confidence != "high" {
		t.Fatalf("bbox points %+v truncated=%v err %v", pts, truncated, err)
	}
	pts, _, err = repo.Points(ctx, domain.FireQuery{Since: since, BBox: &domain.BBox{MinLon: 179, MinLat: -20, MaxLon: -179, MaxLat: -10}}, 10)
	if err != nil || len(pts) != 2 {
		t.Fatalf("antimeridian %+v %v", pts, err)
	}
	pts, truncated, err = repo.Points(ctx, domain.FireQuery{Since: since}, 2)
	if err != nil || !truncated || len(pts) != 2 || !pts[0].AcquiredAt.Equal(t0) {
		t.Fatalf("limit %+v truncated=%v err %v", pts, truncated, err)
	}
	if _, err := repo.Grid(ctx, domain.FireQuery{}, 0); err == nil {
		t.Error("zero cell accepted")
	}
}

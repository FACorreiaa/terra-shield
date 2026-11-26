package store_test

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestHistoricReplaceGridAndPoints(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	// Q100 is an air base in use (on the bases layer) that is also listed
	// as a fort: the historic layer must not show it.
	if _, err := store.NewBaseRepository(pool).Replace(ctx, []domain.MilitaryBase{
		{QID: "Q100", Name: "Active Fort", Kind: domain.BaseAir, Class: "air base", Lon: 10.1, Lat: 50.1},
	}, nil, at); err != nil {
		t.Fatal(err)
	}
	repo := store.NewHistoricRepository(pool)
	sites := []domain.HistoricSite{
		{QID: "Q1", Name: "Burg A", Kind: domain.HistoricCastle, Class: "castle", Lon: 10.2, Lat: 50.2, Description: "castle", WorldHeritage: true},
		{QID: "Q2", Name: "Burg B", Kind: domain.HistoricCastle, Class: "castle", Lon: 10.3, Lat: 50.3, Sitelinks: 40},
		{QID: "Q3", Name: "Fort C", Kind: domain.HistoricMilitary, Class: "fort", Lon: 10.4, Lat: 50.4},
		{QID: "Q100", Name: "Active Fort", Kind: domain.HistoricMilitary, Class: "fort", Lon: 10.1, Lat: 50.1},
		{QID: "Q5", Name: "Pacific Cemetery", Kind: domain.HistoricMemorial, Class: "war cemetery", Lon: 179.5, Lat: -17},
	}
	n, err := repo.Replace(ctx, sites, map[string]string{"Q1": "DE"}, at)
	if err != nil || n != 5 {
		t.Fatalf("replace %d %v", n, err)
	}
	counts, err := repo.CountByKind(ctx)
	if err != nil || counts[domain.HistoricCastle] != 2 || counts[domain.HistoricMilitary] != 2 || counts[domain.HistoricMemorial] != 1 {
		t.Fatalf("counts %v %v", counts, err)
	}

	cells, err := repo.Grid(ctx, domain.HistoricQuery{}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 || cells[0].Count != 3 || cells[0].Kinds[domain.HistoricCastle] != 2 || cells[0].Kinds[domain.HistoricMilitary] != 1 {
		t.Fatalf("grid (active base left out) %+v", cells)
	}
	if cells[0].Lat != 52.5 || cells[0].Lon != 12.5 {
		t.Errorf("cell center %+v", cells[0])
	}

	// A viewport that cuts a cell still counts the whole cell, and one
	// across the antimeridian keeps the cell beyond it.
	edge, err := repo.Grid(ctx, domain.HistoricQuery{BBox: &domain.BBox{MinLon: 10.35, MinLat: 50.35, MaxLon: 11, MaxLat: 51}}, 5)
	if err != nil || len(edge) != 1 || edge[0].Count != 3 {
		t.Fatalf("edge cell counted whole: %+v %v", edge, err)
	}
	wrap, err := repo.Grid(ctx, domain.HistoricQuery{BBox: &domain.BBox{MinLon: 178, MinLat: -20, MaxLon: -178, MaxLat: -10}}, 5)
	if err != nil || len(wrap) != 1 || wrap[0].Count != 1 || wrap[0].Lon != 177.5 {
		t.Fatalf("antimeridian grid: %+v %v", wrap, err)
	}

	pts, more, err := repo.Points(ctx, domain.HistoricQuery{
		BBox:  &domain.BBox{MinLon: 10, MinLat: 50, MaxLon: 11, MaxLat: 51},
		Kinds: []domain.HistoricKind{domain.HistoricCastle},
	}, 1)
	if err != nil || len(pts) != 1 || !more || pts[0].QID != "Q2" || pts[0].Sitelinks != 40 {
		t.Fatalf("a truncated view keeps the most notable first: %+v more=%v %v", pts, more, err)
	}
	pts, more, err = repo.Points(ctx, domain.HistoricQuery{
		BBox:  &domain.BBox{MinLon: 10, MinLat: 50, MaxLon: 11, MaxLat: 51},
		Kinds: []domain.HistoricKind{domain.HistoricCastle},
	}, 2)
	if err != nil || len(pts) != 2 || pts[1].QID != "Q1" || more || pts[1].ISO2 != "DE" || pts[1].Description != "castle" || !pts[1].UpdatedAt.Equal(at) {
		t.Fatalf("castle points %+v more=%v %v", pts, more, err)
	}
	all, _, err := repo.Points(ctx, domain.HistoricQuery{BBox: &domain.BBox{MinLon: 10, MinLat: 50, MaxLon: 11, MaxLat: 51}}, 10)
	if err != nil || len(all) != 3 {
		t.Fatalf("all kinds, active base left out: %+v %v", all, err)
	}
	across, _, err := repo.Points(ctx, domain.HistoricQuery{BBox: &domain.BBox{MinLon: 170, MinLat: -20, MaxLon: -170, MaxLat: -10}}, 10)
	if err != nil || len(across) != 1 || across[0].QID != "Q5" {
		t.Fatalf("antimeridian %+v %v", across, err)
	}
	whs, _, err := repo.Points(ctx, domain.HistoricQuery{WorldHeritage: true}, 10)
	if err != nil || len(whs) != 1 || whs[0].QID != "Q1" || !whs[0].WorldHeritage {
		t.Fatalf("World Heritage only: %+v %v", whs, err)
	}

	// Next week: Q1 has not moved and keeps its country without a hint;
	// Q2 moved and takes its new hint (the template has no country shapes).
	sites[0].WorldHeritage = false
	sites[1].Lon, sites[1].Lat = 10.31, 50.31
	if _, err := repo.Replace(ctx, sites, map[string]string{"Q2": "FR"}, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, _, err := repo.Points(ctx, domain.HistoricQuery{Kinds: []domain.HistoricKind{domain.HistoricCastle}}, 10)
	if err != nil || len(again) != 2 || again[1].ISO2 != "DE" || again[0].ISO2 != "FR" || again[1].WorldHeritage {
		t.Fatalf("countries carried forward: %+v %v", again, err)
	}
}

package store_test

import (
	"context"
	"os"
	"testing"

	"shield/internal/domain"
	"shield/internal/geo/countries"
	"shield/internal/store"
	"shield/internal/testutil"
)

func countryFixture(t *testing.T) []domain.Country {
	t.Helper()
	f, err := os.Open("../geo/countries/testdata/countries.geojson")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	features, _, err := countries.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return features
}

func TestCountryLoad(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	features := countryFixture(t)
	repo := store.NewCountryRepository(pool)

	n, err := repo.Load(ctx, features)
	if err != nil {
		t.Fatal(err)
	}
	// PT, ES, AU (2 merged), SO (2 merged), XK, ZZ.
	if n != 6 {
		t.Fatalf("loaded %d countries, want 6", n)
	}

	var name, iso3 string
	var parts int
	if err := pool.QueryRow(ctx, `SELECT name, iso3, ST_NumGeometries(geom::geometry) FROM countries WHERE iso2 = 'AU'`).
		Scan(&name, &iso3, &parts); err != nil {
		t.Fatal(err)
	}
	if name != "Australia" || iso3 != "AUS" || parts != 2 {
		t.Fatalf("AU = %q %q %d parts", name, iso3, parts)
	}
	if err := pool.QueryRow(ctx, `SELECT name FROM countries WHERE iso2 = 'SO'`).Scan(&name); err != nil || name != "Somalia" {
		t.Fatalf("SO named %q (err %v), want the larger feature's name", name, err)
	}

	// The self-intersecting fixture was repaired into a valid MultiPolygon.
	var valid bool
	if err := pool.QueryRow(ctx, `SELECT ST_IsValid(geom::geometry) FROM countries WHERE iso2 = 'ZZ'`).Scan(&valid); err != nil || !valid {
		t.Fatalf("ZZ valid=%v err %v", valid, err)
	}

	// ADR 4: a point in Lisbon intersects Portugal and nothing else.
	rows, err := pool.Query(ctx, `SELECT iso2 FROM countries
		WHERE ST_Intersects(geom, ST_GeogFromText('POINT(-9.14 38.72)'))`)
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		hits = append(hits, c)
	}
	if len(hits) != 1 || hits[0] != "PT" {
		t.Fatalf("Lisbon intersects %v", hits)
	}

	// Re-loading is an upsert, not a duplicate.
	if _, err := repo.Load(ctx, features); err != nil {
		t.Fatal(err)
	}
	if c, _ := repo.Count(ctx); c != 6 {
		t.Fatalf("count after reload %d", c)
	}
}

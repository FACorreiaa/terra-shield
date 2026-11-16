package countries_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"shield/internal/geo/countries"
	"shield/internal/testutil"
)

func parseFixture(t *testing.T) ([]countries.Feature, []string) {
	t.Helper()
	f, err := os.Open("testdata/countries.geojson")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	features, skipped, err := countries.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return features, skipped
}

func TestParseAppliesOverrides(t *testing.T) {
	features, skipped := parseFixture(t)
	if len(skipped) != 1 || skipped[0] != "Siachen Glacier" {
		t.Fatalf("skipped %v", skipped)
	}
	codes := map[string]string{}
	for _, f := range features {
		codes[f.Name] = f.ISO2 + "/" + f.ISO3
	}
	for name, want := range map[string]string{
		"Somaliland": "SO/SOM",
		"Kosovo":     "XK/XKX",
		"Portugal":   "PT/PRT",
	} {
		if codes[name] != want {
			t.Errorf("%s = %q, want %q", name, codes[name], want)
		}
	}
}

func TestParseRejectsUnknownCodes(t *testing.T) {
	in := `{"features":[{"properties":{"ADMIN":"Nowhere","ISO_A2_EH":"-99","ISO_A3_EH":"-99"},
		"geometry":{"type":"Point","coordinates":[0,0]}}]}`
	if _, _, err := countries.Parse(strings.NewReader(in)); err == nil {
		t.Fatal("want error for a feature with no ISO codes and no override")
	}
}

func TestLoad(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	features, _ := parseFixture(t)

	n, err := countries.Load(ctx, pool, features)
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
	if _, err := countries.Load(ctx, pool, features); err != nil {
		t.Fatal(err)
	}
	if c, _ := countries.Count(ctx, pool); c != 6 {
		t.Fatalf("count after reload %d", c)
	}
}

func TestFetchVerifiesChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer srv.Close()

	_, err := countries.Fetch(t.Context(), srv.Client(), srv.URL, countries.DatasetSHA256)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err %v, want checksum mismatch", err)
	}
	// The matching hash passes.
	sum := sha256.Sum256([]byte("tampered"))
	body, err := countries.Fetch(t.Context(), srv.Client(), srv.URL, hex.EncodeToString(sum[:]))
	if err != nil || string(body) != "tampered" {
		t.Fatalf("body %q err %v", body, err)
	}
}

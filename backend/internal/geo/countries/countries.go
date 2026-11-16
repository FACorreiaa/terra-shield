// Package countries loads Natural Earth admin-0 boundaries into the
// countries table (ADR 4: an event's countries come from its geometry).
//
// Codes follow ISO 3166-1. Natural Earth's ISO_A2_EH/ISO_A3_EH fields
// already fix France, Norway, Taiwan and Kosovo; the few features with no
// ISO code are mapped to the ISO country that contains them, or skipped
// when ISO assigns none (see overrides). Features sharing a code (Australia
// and its territories) are unioned into one MultiPolygon.
package countries

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pinned dataset: Natural Earth 1:50m admin-0 countries, v5.1.2, public
// domain. Changing the version means changing both values together.
const (
	DatasetURL    = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_admin_0_countries.geojson"
	DatasetSHA256 = "3e458fc036ad0a66411f2c1e6cac49c5d7bfb81cb1123bc513b22511a2b7fdeb"
)

// maxDatasetBytes caps the download; the pinned file is about 3 MB.
const maxDatasetBytes = 32 << 20

// override maps a Natural Earth ADMIN name to ISO codes. An empty ISO2
// skips the feature.
type override struct{ iso2, iso3 string }

var overrides = map[string]override{
	"Somaliland":      {"SO", "SOM"}, // within Somalia in ISO 3166
	"Northern Cyprus": {"CY", "CYP"}, // within Cyprus in ISO 3166
	"Kosovo":          {"XK", "XKX"}, // user-assigned code in common use; NE has no A3
	"Siachen Glacier": {},            // no ISO assignment; events there get no country
}

var (
	iso2Re = regexp.MustCompile(`^[A-Z]{2}$`)
	iso3Re = regexp.MustCompile(`^[A-Z]{3}$`)
)

// Feature is one boundary ready to load.
type Feature struct {
	ISO2, ISO3, Name string
	Geometry         json.RawMessage // GeoJSON geometry, WGS84
}

type geojson struct {
	Features []struct {
		Properties struct {
			Admin   string `json:"ADMIN"`
			Name    string `json:"NAME"`
			ISOA2EH string `json:"ISO_A2_EH"`
			ISOA3EH string `json:"ISO_A3_EH"`
		} `json:"properties"`
		Geometry json.RawMessage `json:"geometry"`
	} `json:"features"`
}

// Parse reads a Natural Earth admin-0 GeoJSON FeatureCollection. skipped
// lists the ADMIN names left out on purpose.
func Parse(r io.Reader) (features []Feature, skipped []string, err error) {
	var fc geojson
	if err := json.NewDecoder(r).Decode(&fc); err != nil {
		return nil, nil, fmt.Errorf("decode geojson: %w", err)
	}
	for _, f := range fc.Features {
		p := f.Properties
		iso2, iso3 := p.ISOA2EH, p.ISOA3EH
		if o, ok := overrides[p.Admin]; ok {
			if o.iso2 == "" {
				skipped = append(skipped, p.Admin)
				continue
			}
			iso2, iso3 = o.iso2, o.iso3
		}
		if !iso2Re.MatchString(iso2) || !iso3Re.MatchString(iso3) {
			return nil, nil, fmt.Errorf("feature %q has no usable ISO codes (%q, %q); add an override", p.Admin, iso2, iso3)
		}
		if len(f.Geometry) == 0 || string(f.Geometry) == "null" {
			return nil, nil, fmt.Errorf("feature %q has no geometry", p.Admin)
		}
		name := p.Name
		if name == "" {
			name = p.Admin
		}
		features = append(features, Feature{ISO2: iso2, ISO3: iso3, Name: name, Geometry: f.Geometry})
	}
	if len(features) == 0 {
		return nil, nil, errors.New("no features")
	}
	return features, skipped, nil
}

// Load upserts features into countries in one transaction and returns the
// number of countries written. Geometries are repaired with ST_MakeValid;
// features sharing an ISO2 are unioned, and the largest one names the row.
func Load(ctx context.Context, pool *pgxpool.Pool, features []Feature) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE countries_stage (
		iso2 text NOT NULL, iso3 text NOT NULL, name text NOT NULL, geom geometry NOT NULL
	) ON COMMIT DROP`); err != nil {
		return 0, err
	}
	batch := &pgx.Batch{}
	for _, f := range features {
		batch.Queue(`INSERT INTO countries_stage (iso2, iso3, name, geom)
			VALUES ($1, $2, $3, ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($4), 4326)))`,
			f.ISO2, f.ISO3, f.Name, string(f.Geometry))
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return 0, fmt.Errorf("stage features: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO countries (iso2, iso3, name, geom)
		SELECT iso2,
		       (array_agg(iso3 ORDER BY ST_Area(geom) DESC))[1],
		       (array_agg(name ORDER BY ST_Area(geom) DESC))[1],
		       ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_Union(geom)), 3))::geography
		FROM countries_stage
		GROUP BY iso2
		ON CONFLICT (iso2) DO UPDATE SET
		    iso3 = EXCLUDED.iso3,
		    name = EXCLUDED.name,
		    geom = EXCLUDED.geom`)
	if err != nil {
		return 0, fmt.Errorf("upsert countries: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// Count returns how many countries are loaded.
func Count(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM countries`).Scan(&n)
	return n, err
}

// Fetch downloads url and verifies its SHA-256, so a moved or altered file
// fails loudly instead of loading wrong borders.
func Fetch(ctx context.Context, client *http.Client, url, wantSHA256 string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "shield-seed/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDatasetBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDatasetBytes {
		return nil, fmt.Errorf("GET %s: body over %d bytes", url, maxDatasetBytes)
	}
	if err := Verify(body, wantSHA256); err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	return body, nil
}

// Verify checks body against a hex SHA-256.
func Verify(body []byte, wantSHA256 string) error {
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		return fmt.Errorf("sha256 %s, want %s", got, wantSHA256)
	}
	return nil
}

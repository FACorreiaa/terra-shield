package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"shield/internal/domain"
	"shield/internal/store/nwszonesdb"
)

// NWSZoneRepository holds the NWS public forecast zones (migration 00007).
type NWSZoneRepository struct {
	pool *pgxpool.Pool
	q    *nwszonesdb.Queries
}

// NewNWSZoneRepository builds an NWSZoneRepository.
func NewNWSZoneRepository(pool *pgxpool.Pool) *NWSZoneRepository {
	return &NWSZoneRepository{pool: pool, q: nwszonesdb.New(pool)}
}

// Count returns how many zones are loaded.
func (r *NWSZoneRepository) Count(ctx context.Context) (int, error) {
	n, err := r.q.CountNWSZones(ctx)
	return int(n), err
}

// Load replaces every zone in one transaction and records the dataset
// version. A shapefile marks a ring's role by its direction (outer rings
// clockwise, holes counter-clockwise), so each zone is the union of its
// clockwise rings minus its counter-clockwise ones, each ring repaired on
// its own first: ST_BuildArea on the raw linework fails on the odd
// self-touching coastline in the 2026 file. The result is simplified to
// simplify degrees (when > 0) with its topology kept.
func (r *NWSZoneRepository) Load(ctx context.Context, zones []domain.ForecastZone, simplify float64, name, version string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `TRUNCATE nws_zones`); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, z := range zones {
		batch.Queue(zoneInsert, z.UGC, z.State, z.Name, string(z.Rings), simplify)
	}
	batch.Queue(`INSERT INTO geo_datasets (name, version) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, loaded_at = now()`, name, version)
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("load nws zones: %w", err)
	}
	return tx.Commit(ctx)
}

const zoneInsert = `
WITH rings AS (
    SELECT ST_MakeValid(ST_MakePolygon(d.geom)) AS poly, ST_IsPolygonCW(ST_MakePolygon(d.geom)) AS is_outer
    FROM ST_Dump(ST_SetSRID(ST_GeomFromGeoJSON($4), 4326)) AS d
), shape AS (
    SELECT ST_Difference(
        COALESCE((SELECT ST_Union(poly) FROM rings WHERE is_outer), ST_SetSRID('POLYGON EMPTY'::geometry, 4326)),
        COALESCE((SELECT ST_Union(poly) FROM rings WHERE NOT is_outer), ST_SetSRID('POLYGON EMPTY'::geometry, 4326))
    ) AS g
)
INSERT INTO nws_zones (ugc, state, name, geom)
SELECT $1, $2, $3, ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SimplifyPreserveTopology(g, $5)), 3))
FROM shape`

// Union dissolves the named zones into one MultiPolygon and reports which
// of them we hold. It returns a nil geometry when none matched.
func (r *NWSZoneRepository) Union(ctx context.Context, ugcs []string) (json.RawMessage, []string, error) {
	if len(ugcs) == 0 {
		return nil, nil, errors.New("union nws zones: no zones")
	}
	row, err := r.q.UnionNWSZones(ctx, ugcs)
	if err != nil {
		return nil, nil, fmt.Errorf("union nws zones: %w", err)
	}
	if row.Geometry == "" {
		return nil, nil, nil
	}
	return json.RawMessage(row.Geometry), row.Found, nil
}

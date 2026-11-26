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

// NWSZoneRepository holds the NWS zone sets: public forecast zones
// (migration 00007), fire weather zones and counties (00016).
type NWSZoneRepository struct {
	pool *pgxpool.Pool
	q    *nwszonesdb.Queries
}

// NewNWSZoneRepository builds an NWSZoneRepository.
func NewNWSZoneRepository(pool *pgxpool.Pool) *NWSZoneRepository {
	return &NWSZoneRepository{pool: pool, q: nwszonesdb.New(pool)}
}

// Count returns how many zones of a set are loaded.
func (r *NWSZoneRepository) Count(ctx context.Context, set string) (int, error) {
	n, err := r.q.CountNWSZones(ctx, set)
	return int(n), err
}

// Load replaces every zone of a set in one transaction and records the dataset
// version. A shapefile marks a ring's role by its direction (outer rings
// clockwise, holes counter-clockwise), so each zone is the union of its
// clockwise rings minus its counter-clockwise ones, each ring repaired on
// its own first: ST_BuildArea on the raw linework fails on the odd
// self-touching coastline in the 2026 file. The result is simplified to
// simplify degrees (when > 0) with its topology kept.
func (r *NWSZoneRepository) Load(ctx context.Context, set string, zones []domain.ForecastZone, simplify float64, name, version string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM nws_zones WHERE zone_set = $1`, set); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, z := range zones {
		batch.Queue(zoneInsert, z.UGC, z.State, z.Name, string(z.Rings), simplify, set)
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
INSERT INTO nws_zones (ugc, state, name, geom, zone_set)
SELECT $1, $2, $3, ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SimplifyPreserveTopology(g, $5)), 3)), $6
FROM shape`

// Draw dissolves an alert's area into one MultiPolygon: the zones it names
// (of any set) and the polygons it draws itself. Parts are unioned in a
// fixed order and rounded to 6 decimals (about 0.1 m), so an unchanged
// alert yields the same geometry every poll. found lists the zones held,
// as "set/UGC"; geometry is nil when nothing could be drawn.
func (r *NWSZoneRepository) Draw(ctx context.Context, zones []domain.ZoneRef, polygons []json.RawMessage) (json.RawMessage, []string, error) {
	sets := make([]string, len(zones))
	ugcs := make([]string, len(zones))
	for i, z := range zones {
		sets[i], ugcs[i] = z.Set, z.UGC
	}
	polys := make([]string, len(polygons))
	for i, p := range polygons {
		polys[i] = string(p)
	}
	var geom string
	var found []string
	err := r.pool.QueryRow(ctx, drawSQL, sets, ugcs, polys).Scan(&geom, &found)
	if err != nil {
		return nil, nil, fmt.Errorf("draw nws area: %w", err)
	}
	if geom == "" { // nothing named is held, or what was sent has no area
		return nil, found, nil
	}
	return json.RawMessage(geom), found, nil
}

const drawSQL = `
WITH parts AS (
    SELECT 'z' || z.zone_set || '/' || z.ugc AS k, z.geom, z.zone_set || '/' || z.ugc AS found
    FROM nws_zones z
    JOIN unnest($1::text[], $2::text[]) AS r(zone_set, ugc) ON r.zone_set = z.zone_set AND r.ugc = z.ugc
    UNION ALL
    SELECT 'p' || p, ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON(p), 4326)), NULL
    FROM unnest($3::text[]) AS p
)
-- Snapped to the 6-decimal grid it is sent at, so the shape stays valid
-- once written out: rounding a dense coastline afterwards made it invalid,
-- and repairing it on upsert left a GeometryCollection.
, area AS (
    SELECT ST_Multi(ST_CollectionExtract(ST_ReducePrecision(ST_MakeValid(ST_Union(geom ORDER BY k)), 0.000001), 3)) AS g,
           COALESCE(array_agg(found ORDER BY found) FILTER (WHERE found IS NOT NULL), '{}') AS found
    FROM parts
)
SELECT CASE WHEN g IS NULL OR ST_IsEmpty(g) THEN '' ELSE ST_AsGeoJSON(g, 6) END, found
FROM area`

// Union dissolves the named forecast zones into one MultiPolygon and reports which
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

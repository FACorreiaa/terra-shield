package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"shield/internal/domain"
	"shield/internal/store/warningareasdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WarningAreaRepository holds MeteoAlarm's warning areas and their aliases
// (S1.5), and draws a warning's shape from its codes.
type WarningAreaRepository struct {
	pool *pgxpool.Pool
	q    *warningareasdb.Queries
}

// NewWarningAreaRepository builds a WarningAreaRepository.
func NewWarningAreaRepository(pool *pgxpool.Pool) *WarningAreaRepository {
	return &WarningAreaRepository{pool: pool, q: warningareasdb.New(pool)}
}

// DatasetVersion is the version the named dataset was last loaded at;
// "" when it never was.
func (r *WarningAreaRepository) DatasetVersion(ctx context.Context, name string) (string, error) {
	v, err := r.q.GetDatasetVersion(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// Count returns how many areas are loaded.
func (r *WarningAreaRepository) Count(ctx context.Context) (int, error) {
	n, err := r.q.CountWarningAreas(ctx)
	return int(n), err
}

// Load replaces every area and alias in one transaction and records the
// dataset versions, so readers see the old set or the new one, never a
// mix. Geometries are repaired with ST_MakeValid and, when simplify > 0,
// simplified to that tolerance in degrees with their topology kept.
func (r *WarningAreaRepository) Load(ctx context.Context, areas []domain.WarningArea, aliases []domain.AreaAlias, simplify float64, versions map[string]string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `TRUNCATE meteoalarm_area_aliases, meteoalarm_areas`); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, a := range areas {
		batch.Queue(`INSERT INTO meteoalarm_areas (code, country, name, geom)
			VALUES ($1, $2, $3, ST_Multi(ST_CollectionExtract(ST_MakeValid(
				ST_SimplifyPreserveTopology(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($4), 4326)), $5)), 3)))`,
			a.Code, a.Country, a.Name, string(a.Geometry), simplify)
	}
	for _, a := range aliases {
		batch.Queue(`INSERT INTO meteoalarm_area_aliases (alias_type, alias_code, code) VALUES ($1, $2, $3)`,
			a.Type, a.Alias, a.Code)
	}
	for name, v := range versions {
		batch.Queue(`INSERT INTO geo_datasets (name, version) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, loaded_at = now()`, name, v)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("load warning areas: %w", err)
	}
	return tx.Commit(ctx)
}

// Codes returns the EMMA_IDs behind each geocode: the code itself, or what
// its alias stands for. Geocodes we do not know are absent from the map.
func (r *WarningAreaRepository) Codes(ctx context.Context, geocodes []domain.AreaCode) (map[domain.AreaCode][]string, error) {
	p := warningareasdb.WarningAreaCodesParams{
		Types:  make([]string, len(geocodes)),
		Values: make([]string, len(geocodes)),
	}
	for i, g := range geocodes {
		p.Types[i], p.Values[i] = g.Type, g.Value
	}
	rows, err := r.q.WarningAreaCodes(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("warning area codes: %w", err)
	}
	out := make(map[domain.AreaCode][]string, len(rows))
	for _, row := range rows {
		k := domain.AreaCode{Type: row.Type, Value: row.Value}
		out[k] = append(out[k], row.Code)
	}
	return out, nil
}

// Union dissolves the areas and inline polygons into one MultiPolygon.
// At least one part is required.
func (r *WarningAreaRepository) Union(ctx context.Context, codes []string, polygons []json.RawMessage) (json.RawMessage, error) {
	if len(codes) == 0 && len(polygons) == 0 {
		return nil, errors.New("union warning areas: no parts")
	}
	polys := make([]string, len(polygons))
	for i, p := range polygons {
		polys[i] = string(p)
	}
	geom, err := r.q.UnionWarningAreas(ctx, warningareasdb.UnionWarningAreasParams{Codes: codes, Polygons: polys})
	if err != nil {
		return nil, fmt.Errorf("union warning areas: %w", err)
	}
	if geom == "" {
		return nil, errors.New("union warning areas: no area matched")
	}
	return json.RawMessage(geom), nil
}

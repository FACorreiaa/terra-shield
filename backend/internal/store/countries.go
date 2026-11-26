package store

import (
	"context"
	"fmt"

	"shield/internal/domain"
	"shield/internal/store/countriesdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CountryRepository holds admin-0 boundaries (ADR 4).
type CountryRepository struct {
	pool *pgxpool.Pool
}

// NewCountryRepository builds a CountryRepository.
func NewCountryRepository(pool *pgxpool.Pool) *CountryRepository {
	return &CountryRepository{pool: pool}
}

// Load upserts countries in one transaction and returns how many rows were
// written. Geometries are repaired with ST_MakeValid; entries sharing an
// ISO2 are unioned, and the largest one names the row.
func (r *CountryRepository) Load(ctx context.Context, countries []domain.Country) (int, error) {
	tx, err := r.pool.Begin(ctx)
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
	for _, c := range countries {
		batch.Queue(`INSERT INTO countries_stage (iso2, iso3, name, geom)
			VALUES ($1, $2, $3, ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($4), 4326)))`,
			c.ISO2, c.ISO3, c.Name, string(c.Geometry))
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return 0, fmt.Errorf("stage countries: %w", err)
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
	// The historic layer keeps a site's country from poll to poll while it
	// has not moved (HistoricRepository.Replace); new borders clear them,
	// so its next poll places every site again.
	if _, err := tx.Exec(ctx, `UPDATE historic_sites SET iso2 = NULL WHERE iso2 IS NOT NULL`); err != nil {
		return 0, fmt.Errorf("clear historic countries: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// Count returns how many countries are loaded.
func (r *CountryRepository) Count(ctx context.Context) (int, error) {
	n, err := countriesdb.New(r.pool).CountCountries(ctx)
	return int(n), err
}

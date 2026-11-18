package store

import (
	"context"
	"fmt"
	"time"

	"shield/internal/domain"
	"shield/internal/store/firesdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FireRepository holds FIRMS detections, a rolling window (ADR 10).
type FireRepository struct {
	pool *pgxpool.Pool
}

// NewFireRepository builds a FireRepository.
func NewFireRepository(pool *pgxpool.Pool) *FireRepository {
	return &FireRepository{pool: pool}
}

var fireStageColumns = []string{"satellite", "acq_at", "lat", "lon", "frp", "confidence"}

// Insert adds detections not already stored and returns how many were new.
// A FIRMS file is a sliding 24 h window, so each poll repeats most rows:
// they are COPYed into a staging table, and only the new ones are inserted
// and given a country. Country lookup runs for new rows only, because it
// is the expensive part.
func (r *FireRepository) Insert(ctx context.Context, fires []domain.FireDetection) (int, error) {
	if len(fires) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE fire_stage (
		satellite text NOT NULL, acq_at timestamptz NOT NULL,
		lat double precision NOT NULL, lon double precision NOT NULL,
		frp real, confidence text
	) ON COMMIT DROP`); err != nil {
		return 0, err
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"fire_stage"}, fireStageColumns,
		pgx.CopyFromSlice(len(fires), func(i int) ([]any, error) {
			f := &fires[i]
			return []any{f.Satellite, f.AcquiredAt, f.Lat, f.Lon, f.FRP, nonEmpty(f.Confidence)}, nil
		})); err != nil {
		return 0, fmt.Errorf("copy fire detections: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO fire_detections (satellite, acq_at, lat, lon, frp, confidence, iso2)
		SELECT DISTINCT ON (s.satellite, s.acq_at, s.lat, s.lon)
		       s.satellite, s.acq_at, s.lat, s.lon, s.frp, s.confidence,
		       (SELECT c.iso2 FROM countries c
		        WHERE ST_Intersects(c.geom, ST_SetSRID(ST_MakePoint(s.lon, s.lat), 4326)::geography)
		        LIMIT 1)
		FROM fire_stage s
		WHERE NOT EXISTS (
		    SELECT 1 FROM fire_detections f
		    WHERE f.satellite = s.satellite AND f.acq_at = s.acq_at AND f.lat = s.lat AND f.lon = s.lon)
		ON CONFLICT (satellite, acq_at, lat, lon) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("insert fire detections: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// PurgeBefore deletes detections acquired before t.
func (r *FireRepository) PurgeBefore(ctx context.Context, t time.Time) (int, error) {
	n, err := firesdb.New(r.pool).PurgeFireDetections(ctx, t)
	return int(n), err
}

// Count returns how many detections are stored.
func (r *FireRepository) Count(ctx context.Context) (int, error) {
	n, err := firesdb.New(r.pool).CountFireDetections(ctx)
	return int(n), err
}

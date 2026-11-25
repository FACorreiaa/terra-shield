package store

import (
	"context"
	"fmt"
	"time"

	"shield/internal/domain"
	"shield/internal/store/basesdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BaseRepository stores the military bases layer (L3, ADR 29).
type BaseRepository struct {
	pool *pgxpool.Pool
	q    *basesdb.Queries
}

// NewBaseRepository builds a BaseRepository on pool.
func NewBaseRepository(pool *pgxpool.Pool) *BaseRepository {
	return &BaseRepository{pool: pool, q: basesdb.New(pool)}
}

var baseColumns = []string{"qid", "name", "description", "kind", "class", "operator", "lat", "lon", "country_hint", "updated_at"}

// Replace swaps the whole layer for bases in one transaction, so readers
// see the old set or the new one, never a mix. Countries come from
// Shield's own geometry (ADR 4), with Wikidata's country as the fallback.
// hints maps QID to that fallback ISO code.
func (r *BaseRepository) Replace(ctx context.Context, bases []domain.MilitaryBase, hints map[string]string, at time.Time) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM military_bases`); err != nil {
		return 0, err
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{"military_bases"}, baseColumns,
		pgx.CopyFromSlice(len(bases), func(i int) ([]any, error) {
			b := &bases[i]
			return []any{
				b.QID, b.Name, nonEmpty(b.Description), string(b.Kind), b.Class, nonEmpty(b.Operator),
				b.Lat, b.Lon, nonEmpty(hints[b.QID]), at,
			}, nil
		}))
	if err != nil {
		return 0, fmt.Errorf("copy military bases: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE military_bases b SET iso2 = COALESCE(
		    (SELECT c.iso2 FROM countries c
		     WHERE ST_Intersects(c.geom, ST_SetSRID(ST_MakePoint(b.lon, b.lat), 4326)::geography)
		     LIMIT 1),
		    b.country_hint)`); err != nil {
		return 0, fmt.Errorf("assign base countries: %w", err)
	}
	return int(n), tx.Commit(ctx)
}

// Count returns how many bases are stored.
func (r *BaseRepository) Count(ctx context.Context) (int, error) {
	n, err := r.q.CountMilitaryBases(ctx)
	return int(n), err
}

// InBox returns the bases inside bb, or all of them when bb is nil.
func (r *BaseRepository) InBox(ctx context.Context, bb *domain.BBox) ([]domain.MilitaryBase, error) {
	p := basesdb.MilitaryBasesInBoxParams{MinLat: -90, MaxLat: 90, MinLon: -180, MaxLon: 180}
	if bb != nil {
		p = basesdb.MilitaryBasesInBoxParams{MinLat: bb.MinLat, MaxLat: bb.MaxLat, MinLon: bb.MinLon, MaxLon: bb.MaxLon}
	}
	rows, err := r.q.MilitaryBasesInBox(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MilitaryBase, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		out = append(out, domain.MilitaryBase{
			QID: row.Qid, Name: row.Name, Description: deref(row.Description), Kind: domain.BaseKind(row.Kind),
			Class: row.Class, Operator: deref(row.Operator), Lon: row.Lon, Lat: row.Lat, ISO2: deref(row.Iso2),
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

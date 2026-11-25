package store

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"shield/internal/domain"
	"shield/internal/store/historicdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HistoricRepository stores the historic layer (L4, ADR 30).
type HistoricRepository struct {
	pool *pgxpool.Pool
	q    *historicdb.Queries
}

// NewHistoricRepository builds a HistoricRepository on pool.
func NewHistoricRepository(pool *pgxpool.Pool) *HistoricRepository {
	return &HistoricRepository{pool: pool, q: historicdb.New(pool)}
}

var historicColumns = []string{"qid", "name", "description", "kind", "class", "lat", "lon", "country_hint", "updated_at"}

// Replace swaps the whole layer for sites in one transaction. Countries
// come from Shield's own geometry, with hints (QID to ISO code, from
// Wikidata) as the fallback.
func (r *HistoricRepository) Replace(ctx context.Context, sites []domain.HistoricSite, hints map[string]string, at time.Time) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM historic_sites`); err != nil {
		return 0, err
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{"historic_sites"}, historicColumns,
		pgx.CopyFromSlice(len(sites), func(i int) ([]any, error) {
			s := &sites[i]
			return []any{s.QID, s.Name, nonEmpty(s.Description), string(s.Kind), s.Class, s.Lat, s.Lon, nonEmpty(hints[s.QID]), at}, nil
		}))
	if err != nil {
		return 0, fmt.Errorf("copy historic sites: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE historic_sites h SET iso2 = COALESCE(
		    (SELECT c.iso2 FROM countries c
		     WHERE ST_Intersects(c.geom, ST_SetSRID(ST_MakePoint(h.lon, h.lat), 4326)::geography)
		     LIMIT 1),
		    h.country_hint)`); err != nil {
		return 0, fmt.Errorf("assign historic site countries: %w", err)
	}
	return int(n), tx.Commit(ctx)
}

// CountByKind returns how many sites of each kind are stored.
func (r *HistoricRepository) CountByKind(ctx context.Context) (map[domain.HistoricKind]int, error) {
	rows, err := r.q.CountHistoricSitesByKind(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.HistoricKind]int, len(rows))
	for _, row := range rows {
		out[domain.HistoricKind(row.Kind)] = int(row.N)
	}
	return out, nil
}

// historicFilters: kinds, the viewport (split across the antimeridian),
// and never an installation that is on the bases layer, i.e. in use:
// those are L3's, and the historic layer is everything else (ADR 30).
func historicFilters(b *sqlBuilder, q *domain.HistoricQuery) string {
	where := "\nWHERE NOT EXISTS (SELECT 1 FROM military_bases m WHERE m.qid = h.qid)"
	if len(q.Kinds) > 0 {
		where += "\n  AND h.kind = ANY(" + b.arg(strs(q.Kinds)) + ")"
	}
	if bb := q.BBox; bb != nil {
		where += "\n  AND h.lat BETWEEN " + b.arg(bb.MinLat) + " AND " + b.arg(bb.MaxLat)
		if bb.MinLon <= bb.MaxLon {
			where += "\n  AND h.lon BETWEEN " + b.arg(bb.MinLon) + " AND " + b.arg(bb.MaxLon)
		} else {
			where += "\n  AND (h.lon >= " + b.arg(bb.MinLon) + " OR h.lon <= " + b.arg(bb.MaxLon) + ")"
		}
	}
	return where
}

// Grid counts sites per cell of cellDegrees and kind, largest cell first.
// Cells are aligned to multiples of cellDegrees, so they stay put while
// the viewport pans.
func (r *HistoricRepository) Grid(ctx context.Context, q domain.HistoricQuery, cellDegrees float64) ([]domain.HistoricCell, error) {
	if cellDegrees <= 0 {
		return nil, fmt.Errorf("historic grid: cell size must be positive")
	}
	var b sqlBuilder
	cell := b.arg(cellDegrees)
	where := historicFilters(&b, &q)
	rows, err := r.pool.Query(ctx, `
SELECT floor(h.lat / `+cell+`)::bigint AS gy, floor(h.lon / `+cell+`)::bigint AS gx, h.kind, count(*)::integer
FROM historic_sites h`+where+`
GROUP BY gy, gx, h.kind`, b.args...)
	if err != nil {
		return nil, fmt.Errorf("historic grid: %w", err)
	}
	defer rows.Close()
	type key struct{ gy, gx int64 }
	cells := map[key]*domain.HistoricCell{}
	var order []key
	for rows.Next() {
		var (
			k    key
			kind string
			n    int
		)
		if err := rows.Scan(&k.gy, &k.gx, &kind, &n); err != nil {
			return nil, fmt.Errorf("historic grid: %w", err)
		}
		c := cells[k]
		if c == nil {
			c = &domain.HistoricCell{
				Lat: (float64(k.gy) + 0.5) * cellDegrees, Lon: (float64(k.gx) + 0.5) * cellDegrees,
				Kinds: map[domain.HistoricKind]int{},
			}
			cells[k] = c
			order = append(order, k)
		}
		c.Count += n
		c.Kinds[domain.HistoricKind(kind)] += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.HistoricCell, 0, len(order))
	for _, k := range order {
		out = append(out, *cells[k])
	}
	sortCells(out)
	return out, nil
}

// Points returns up to limit sites, by QID, and whether more matched.
func (r *HistoricRepository) Points(ctx context.Context, q domain.HistoricQuery, limit int) ([]domain.HistoricSite, bool, error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("historic points: limit must be positive")
	}
	var b sqlBuilder
	where := historicFilters(&b, &q)
	rows, err := r.pool.Query(ctx, `
SELECT h.qid, h.name, COALESCE(h.description, ''), h.kind, h.class, h.lat, h.lon, COALESCE(h.iso2, ''), h.updated_at
FROM historic_sites h`+where+`
ORDER BY h.qid
LIMIT `+b.arg(limit+1), b.args...)
	if err != nil {
		return nil, false, fmt.Errorf("historic points: %w", err)
	}
	defer rows.Close()
	var out []domain.HistoricSite
	for rows.Next() {
		var s domain.HistoricSite
		var kind string
		if err := rows.Scan(&s.QID, &s.Name, &s.Description, &kind, &s.Class, &s.Lat, &s.Lon, &s.ISO2, &s.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("historic points: %w", err)
		}
		s.Kind = domain.HistoricKind(kind)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

// sortCells orders cells largest first, then south to north, west to east,
// so answers are stable for ETags.
func sortCells(cells []domain.HistoricCell) {
	slices.SortFunc(cells, func(a, b domain.HistoricCell) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		if c := cmp.Compare(a.Lat, b.Lat); c != 0 {
			return c
		}
		return cmp.Compare(a.Lon, b.Lon)
	})
}

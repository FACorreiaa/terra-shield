package store

import (
	"context"
	"fmt"
	"time"

	"shield/internal/domain"
)

// Grid aggregates detections into cells of cellDegrees, largest count
// first. Cells are aligned to multiples of cellDegrees, so they stay put
// while the viewport pans.
func (r *FireRepository) Grid(ctx context.Context, q domain.FireQuery, cellDegrees float64) ([]domain.FireCell, error) {
	if cellDegrees <= 0 {
		return nil, fmt.Errorf("fire grid: cell size must be positive")
	}
	var b sqlBuilder
	cell := b.arg(cellDegrees)
	where := fireFilters(&b, &q)
	rows, err := r.pool.Query(ctx, `
SELECT floor(lat / `+cell+`)::bigint AS gy, floor(lon / `+cell+`)::bigint AS gx,
       count(*)::integer, max(frp), max(acq_at)
FROM fire_detections`+where+`
GROUP BY gy, gx
ORDER BY count(*) DESC, gy, gx`, b.args...)
	if err != nil {
		return nil, fmt.Errorf("fire grid: %w", err)
	}
	defer rows.Close()
	var out []domain.FireCell
	for rows.Next() {
		var (
			gy, gx int64
			c      domain.FireCell
		)
		if err := rows.Scan(&gy, &gx, &c.Count, &c.MaxFRP, &c.LatestAt); err != nil {
			return nil, fmt.Errorf("fire grid: %w", err)
		}
		c.Lat = (float64(gy) + 0.5) * cellDegrees
		c.Lon = (float64(gx) + 0.5) * cellDegrees
		out = append(out, c)
	}
	return out, rows.Err()
}

// Points returns up to limit detections, newest first, and whether more
// matched.
func (r *FireRepository) Points(ctx context.Context, q domain.FireQuery, limit int) ([]domain.FireDetection, bool, error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("fire points: limit must be positive")
	}
	var b sqlBuilder
	where := fireFilters(&b, &q)
	rows, err := r.pool.Query(ctx, `
SELECT satellite, acq_at, lat, lon, frp, COALESCE(confidence, ''), COALESCE(iso2::text, '')
FROM fire_detections`+where+`
ORDER BY acq_at DESC, id DESC
LIMIT `+b.arg(limit+1), b.args...)
	if err != nil {
		return nil, false, fmt.Errorf("fire points: %w", err)
	}
	defer rows.Close()
	var out []domain.FireDetection
	for rows.Next() {
		var d domain.FireDetection
		if err := rows.Scan(&d.Satellite, &d.AcquiredAt, &d.Lat, &d.Lon, &d.FRP, &d.Confidence, &d.ISO2); err != nil {
			return nil, false, fmt.Errorf("fire points: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

// fireFilters: detections are plain lon/lat points, so the viewport is a
// range test on the columns, split in two across the antimeridian.
func fireFilters(b *sqlBuilder, q *domain.FireQuery) string {
	since := q.Since
	if since.IsZero() {
		since = time.Unix(0, 0)
	}
	where := "\nWHERE acq_at >= " + b.arg(since)
	if bb := q.BBox; bb != nil {
		where += "\n  AND lat BETWEEN " + b.arg(bb.MinLat) + " AND " + b.arg(bb.MaxLat)
		if bb.MinLon <= bb.MaxLon {
			where += "\n  AND lon BETWEEN " + b.arg(bb.MinLon) + " AND " + b.arg(bb.MaxLon)
		} else {
			where += "\n  AND (lon >= " + b.arg(bb.MinLon) + " OR lon <= " + b.arg(bb.MaxLon) + ")"
		}
	}
	return where
}

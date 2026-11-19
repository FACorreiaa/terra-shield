package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/store/eventsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// List reads events matching q, most severe first, then most recently
// updated. It returns at most q.Limit records and reports whether more
// matched. Filters are bound parameters; only fixed SQL fragments are
// concatenated.
func (r *EventRepository) List(ctx context.Context, q domain.EventQuery) ([]domain.EventRecord, bool, error) {
	return listEvents(ctx, r.pool, q)
}

// querier is a pool or a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listEvents(ctx context.Context, db querier, q domain.EventQuery) ([]domain.EventRecord, bool, error) {
	if q.Limit <= 0 {
		return nil, false, fmt.Errorf("event query: limit must be positive")
	}
	var b sqlBuilder
	geometry := "NULL::text"
	if q.Geometry {
		g := "e.geom::geometry"
		if q.SimplifyDegrees > 0 {
			g = "ST_SimplifyPreserveTopology(e.geom::geometry, " + b.arg(q.SimplifyDegrees) + ")"
		}
		geometry = "ST_AsGeoJSON(" + g + ", 5)" // 5 decimals ≈ 1 m
	}
	distance, order := "NULL::float8", ""
	if q.Near != nil {
		d := "ST_Distance(e.geom, " + nearPoint(&b, q.Near) + ")"
		distance = "round((" + d + " / 1000)::numeric, 1)::float8"
		order = d + ", "
	}
	where := listFilters(&b, &q)
	sql := `
SELECT ` + recordColumns + `,
       ` + geometry + `,
       ` + distance + `
FROM events e` + where + `
ORDER BY ` + order + `array_position(ARRAY['minor','moderate','severe','extreme'], e.severity) DESC, e.updated_at DESC, e.id
LIMIT ` + b.arg(q.Limit+1)

	rows, err := db.Query(ctx, sql, b.args...)
	if err != nil {
		return nil, false, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var out []domain.EventRecord
	for rows.Next() {
		var dist *float64
		rec, err := scanRecord(rows, &dist)
		if err != nil {
			return nil, false, err
		}
		rec.DistanceKm = dist
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("list events: %w", err)
	}
	if len(out) > q.Limit {
		return out[:q.Limit], true, nil
	}
	return out, false, nil
}

// recordColumns are what scanRecord reads, in order, from events e.
const recordColumns = `e.id::text, e.source_id, e.source_event_id, e.type, e.kind, e.status, e.severity,
       e.source_severity, e.title, e.summary,
       ST_Y(e.centroid::geometry), ST_X(e.centroid::geometry),
       e.started_at, e.observed_at, e.updated_at, e.ended_at, e.fetched_at, e.url, e.attrs,
       COALESCE((SELECT json_agg(json_build_object('iso2', ec.iso2, 'relation', ec.relation, 'distanceKm', ec.distance_km)
                                 ORDER BY ec.relation = 'nearby', ec.distance_km, ec.iso2)
                 FROM event_countries ec WHERE ec.event_id = e.id), '[]'::json)`

// maxRevisions bounds the history returned with an event.
const maxRevisions = 50

// Detail reads one event by id with its exact geometry, its source and its
// newest revisions. A missing id is domain.ErrNotFound.
func (r *EventRepository) Detail(ctx context.Context, id uuid.UUID) (domain.EventDetail, error) {
	var (
		d           domain.EventDetail
		pollSeconds int32
		lastError   *string
	)
	src := &d.Source
	row := r.pool.QueryRow(ctx, `
SELECT `+recordColumns+`,
       ST_AsGeoJSON(e.geom::geometry, 6),
       s.id, s.name, s.url, s.license, s.attribution, s.poll_interval_seconds, s.enabled,
       s.last_success_at, s.last_error, s.last_error_at
FROM events e
JOIN sources s ON s.id = e.source_id
WHERE e.id = $1`, id)
	rec, err := scanRecord(row,
		&src.ID, &src.Name, &src.URL, &src.License, &src.Attribution, &pollSeconds, &src.Enabled,
		&src.LastSuccessAt, &lastError, &src.LastErrorAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, domain.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.EventRecord = rec
	src.PollInterval = time.Duration(pollSeconds) * time.Second
	src.LastError = deref(lastError)

	revs, err := eventsdb.New(r.pool).ListEventRevisions(ctx, eventsdb.ListEventRevisionsParams{EventID: id, MaxRows: maxRevisions})
	if err != nil {
		return d, fmt.Errorf("event %s revisions: %w", id, err)
	}
	d.Revisions = make([]domain.Revision, 0, len(revs))
	for _, rv := range revs {
		var changes map[string]any
		if err := json.Unmarshal(rv.Changes, &changes); err != nil {
			return d, fmt.Errorf("event %s revision: %w", id, err)
		}
		d.Revisions = append(d.Revisions, domain.Revision{At: rv.At, Changes: changes})
	}
	return d, nil
}

func listFilters(b *sqlBuilder, q *domain.EventQuery) string {
	var where []string
	if q.BBox != nil {
		bb := q.BBox
		env := func(minLon, maxLon float64) string {
			return "ST_Intersects(e.geom::geometry, ST_MakeEnvelope(" +
				b.arg(minLon) + ", " + b.arg(bb.MinLat) + ", " + b.arg(maxLon) + ", " + b.arg(bb.MaxLat) + ", 4326))"
		}
		if bb.MinLon <= bb.MaxLon {
			where = append(where, env(bb.MinLon, bb.MaxLon))
		} else { // crosses the antimeridian: two rectangles
			where = append(where, "("+env(bb.MinLon, 180)+" OR "+env(-180, bb.MaxLon)+")")
		}
	}
	if len(q.Types) > 0 {
		where = append(where, "e.type = ANY("+b.arg(strs(q.Types))+")")
	}
	if len(q.Severities) > 0 {
		where = append(where, "e.severity = ANY("+b.arg(q.Severities)+")")
	}
	if len(q.Kinds) > 0 {
		where = append(where, "e.kind = ANY("+b.arg(strs(q.Kinds))+")")
	}
	if len(q.Sources) > 0 {
		where = append(where, "e.source_id = ANY("+b.arg(q.Sources)+")")
	}
	if len(q.Statuses) > 0 {
		where = append(where, "e.status = ANY("+b.arg(strs(q.Statuses))+")")
	}
	if q.Country != "" {
		where = append(where, "EXISTS (SELECT 1 FROM event_countries ec WHERE ec.event_id = e.id AND ec.iso2 = "+b.arg(q.Country)+")")
	}
	if q.Near != nil {
		// On geography, so the radius is in metres on the ellipsoid and the
		// GiST index on geom serves it.
		where = append(where, "ST_DWithin(e.geom, "+nearPoint(b, q.Near)+", "+b.arg(q.Near.RadiusKm*1000)+")")
	}
	// The window is about when an event last mattered: its source's last
	// update while active, the time it ended once resolved. A GDACS fire
	// re-sent today that ended four days ago is not "resolved in the last
	// 24 h".
	const windowAt = "(CASE WHEN e.status = 'resolved' THEN coalesce(e.ended_at, e.updated_at) ELSE e.updated_at END)"
	if q.Since != nil {
		where = append(where, windowAt+" >= "+b.arg(*q.Since))
	}
	if q.Until != nil {
		where = append(where, windowAt+" < "+b.arg(*q.Until))
	}
	if len(where) == 0 {
		return ""
	}
	return "\nWHERE " + strings.Join(where, "\n  AND ")
}

// nearPoint is the query point of a near filter, as geography.
func nearPoint(b *sqlBuilder, n *domain.Near) string {
	return "ST_SetSRID(ST_MakePoint(" + b.arg(n.Lon) + ", " + b.arg(n.Lat) + "), 4326)::geography"
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanRecord reads recordColumns plus the geometry column, then any extra
// columns the caller selected after them.
func scanRecord(row rowScanner, extra ...any) (domain.EventRecord, error) {
	var (
		rec                          domain.EventRecord
		typ, kind, status            string
		sourceSeverity, summary, url *string
		attrs, countries             []byte
		geometry                     *string
		startedAt, endedAt           *time.Time
	)
	dest := append([]any{
		&rec.ID, &rec.SourceID, &rec.SourceEventID, &typ, &kind, &status, &rec.Severity,
		&sourceSeverity, &rec.Title, &summary, &rec.Latitude, &rec.Longitude,
		&startedAt, &rec.ObservedAt, &rec.UpdatedAt, &endedAt, &rec.FetchedAt, &url, &attrs,
		&countries, &geometry,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return rec, fmt.Errorf("scan event: %w", err)
	}
	rec.Type, rec.Kind, rec.Status = domain.EventType(typ), domain.Kind(kind), domain.Status(status)
	rec.SourceSeverity, rec.Summary, rec.URL = deref(sourceSeverity), deref(summary), deref(url)
	rec.StartedAt, rec.EndedAt = startedAt, endedAt
	if err := json.Unmarshal(attrs, &rec.Attrs); err != nil {
		return rec, fmt.Errorf("event %s attrs: %w", rec.ID, err)
	}
	if err := json.Unmarshal(countries, &rec.Countries); err != nil {
		return rec, fmt.Errorf("event %s countries: %w", rec.ID, err)
	}
	if geometry != nil {
		rec.Geometry = json.RawMessage(*geometry)
	}
	return rec, nil
}

// sqlBuilder numbers bind parameters as fragments are added.
type sqlBuilder struct {
	args []any
}

func (b *sqlBuilder) arg(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

func strs[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

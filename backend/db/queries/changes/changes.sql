-- name: ListChanges :many
-- What changed since @since::timestamptz, one row per event (BACKLOG queue item 2):
--   ended      active at the window's start (or first seen in it), resolved now
--   new        active now, and first seen in the window or resolved at its start
--   escalated  active now, more severe than at the window's start
--   downgraded active now, less severe than at the window's start
-- The state at the window's start is the "from" of the event's first
-- status and severity revisions in the window, so a round trip is no
-- change: an event a feed blip resolved and relisted is not new. One that
-- arrived and ended inside the window is ended. Geometry-only revisions
-- are out.
-- Rows that changed in none of these ways come back with an empty change.
WITH rev AS (
    -- Each event's revisions in the window, folded once.
    SELECT r.event_id,
           max(r.at) FILTER (WHERE r.changes -> 'status' ->> 'to' = 'resolved') AS ended_at,
           max(r.at) FILTER (WHERE r.changes -> 'status' ->> 'to' = 'active') AS reopened_at,
           (array_agg(r.changes -> 'status' ->> 'from' ORDER BY r.at, r.id)
                FILTER (WHERE r.changes ? 'status'))[1] AS from_status,
           (array_agg(r.changes -> 'severity' ->> 'from' ORDER BY r.at, r.id)
                FILTER (WHERE r.changes ? 'severity'))[1] AS from_severity,
           max(r.at) FILTER (WHERE r.changes ? 'severity') AS severity_at
    FROM event_revisions r
    WHERE r.at >= @since::timestamptz AND (r.changes ? 'severity' OR r.changes ? 'status')
    GROUP BY r.event_id
), cand AS (
    SELECT rev.event_id AS id FROM rev
    UNION
    SELECT ev.id FROM events ev WHERE ev.first_seen_at >= @since::timestamptz
)
SELECT e.id, e.source_id, e.type, e.kind, e.severity, e.source_severity, e.title,
       ST_X(e.centroid::geometry)::float8 AS lon, ST_Y(e.centroid::geometry)::float8 AS lat,
       e.started_at,
       ARRAY(SELECT ec.iso2 FROM event_countries ec WHERE ec.event_id = e.id ORDER BY ec.iso2)::text[] AS countries,
       COALESCE(rv.from_severity, '')::text AS from_severity,
       (CASE
           WHEN e.status = 'resolved' AND rv.ended_at IS NOT NULL AND rv.from_status = 'active' THEN 'ended'
           WHEN e.status = 'active' AND (e.first_seen_at >= @since::timestamptz OR rv.from_status = 'resolved') THEN 'new'
           WHEN e.status = 'active' AND rv.from_severity IS NOT NULL
                AND array_position(ARRAY['minor','moderate','severe','extreme'], rv.from_severity)
                  < array_position(ARRAY['minor','moderate','severe','extreme'], e.severity) THEN 'escalated'
           WHEN e.status = 'active' AND rv.from_severity IS NOT NULL
                AND array_position(ARRAY['minor','moderate','severe','extreme'], rv.from_severity)
                  > array_position(ARRAY['minor','moderate','severe','extreme'], e.severity) THEN 'downgraded'
           ELSE ''
       END)::text AS change,
       COALESCE(CASE
           WHEN e.status = 'resolved' THEN rv.ended_at
           WHEN e.status = 'active' AND (e.first_seen_at >= @since::timestamptz OR rv.from_status = 'resolved')
               THEN GREATEST(CASE WHEN e.first_seen_at >= @since::timestamptz THEN e.first_seen_at END,
                             CASE WHEN rv.from_status = 'resolved' THEN rv.reopened_at END)
           ELSE rv.severity_at
       END, e.updated_at)::timestamptz AS at
FROM cand c
JOIN events e ON e.id = c.id
LEFT JOIN rev rv ON rv.event_id = e.id
WHERE e.type = ANY(@types::text[])
  AND (cardinality(@severities::text[]) = 0 OR e.severity = ANY(@severities::text[]))
  AND (sqlc.narg('iso2')::text IS NULL
       OR EXISTS (SELECT 1 FROM event_countries ec WHERE ec.event_id = e.id AND ec.iso2 = sqlc.narg('iso2')::text));

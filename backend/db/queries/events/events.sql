-- name: UpsertEvent :one
-- Inserts or updates one event and returns its id only when something
-- changed, so unchanged polls cost no writes and no country reassignment.
-- Geometry arrives as GeoJSON; Z (USGS depth) is dropped, depth lives in attrs.
INSERT INTO events (
    source_id, source_event_id, type, kind, status, severity, source_severity,
    title, summary, geom, centroid, started_at, observed_at, updated_at, ended_at,
    fetched_at, url, attrs
) VALUES (
    @source_id, @source_event_id, @type, @kind, @status, @severity, @source_severity,
    @title, @summary,
    ST_Force2D(ST_GeomFromGeoJSON(@geom::text))::geography,
    ST_PointOnSurface(ST_Force2D(ST_GeomFromGeoJSON(@geom::text)))::geography,
    @started_at, @observed_at, @updated_at, @ended_at,
    now(), @url, @attrs
)
ON CONFLICT (source_id, source_event_id) DO UPDATE SET
    type            = EXCLUDED.type,
    kind            = EXCLUDED.kind,
    status          = EXCLUDED.status,
    severity        = EXCLUDED.severity,
    source_severity = EXCLUDED.source_severity,
    title           = EXCLUDED.title,
    summary         = EXCLUDED.summary,
    geom            = EXCLUDED.geom,
    centroid        = EXCLUDED.centroid,
    started_at      = EXCLUDED.started_at,
    observed_at     = EXCLUDED.observed_at,
    updated_at      = EXCLUDED.updated_at,
    ended_at        = EXCLUDED.ended_at,
    fetched_at      = now(),
    url             = EXCLUDED.url,
    attrs           = EXCLUDED.attrs
WHERE (events.type, events.kind, events.status, events.severity, events.source_severity,
       events.title, events.summary, events.started_at, events.observed_at,
       events.updated_at, events.ended_at, events.url, events.attrs)
      IS DISTINCT FROM
      (EXCLUDED.type, EXCLUDED.kind, EXCLUDED.status, EXCLUDED.severity, EXCLUDED.source_severity,
       EXCLUDED.title, EXCLUDED.summary, EXCLUDED.started_at, EXCLUDED.observed_at,
       EXCLUDED.updated_at, EXCLUDED.ended_at, EXCLUDED.url, EXCLUDED.attrs)
   OR NOT ST_Equals(events.geom::geometry, EXCLUDED.geom::geometry)
RETURNING id;

-- name: ClearEventCountries :exec
DELETE FROM event_countries WHERE event_id = @event_id;

-- name: AssignEventCountries :exec
-- ADR 4: every country the geometry intersects ('within'); when there is
-- none, the nearest country within 250 km ('nearby').
WITH e AS (
    SELECT geom FROM events WHERE id = @event_id
), within AS (
    SELECT c.iso2
    FROM countries c, e
    WHERE ST_Intersects(c.geom, e.geom)
), nearby AS (
    SELECT c.iso2, ST_Distance(c.geom, e.geom) AS meters
    FROM countries c, e
    WHERE NOT EXISTS (SELECT 1 FROM within)
      AND ST_DWithin(c.geom, e.geom, 250000)
    ORDER BY c.geom <-> e.geom
    LIMIT 1
)
INSERT INTO event_countries (event_id, iso2, relation, distance_km)
SELECT @event_id::uuid AS event_id, iso2, 'within' AS relation, 0::real AS distance_km FROM within
UNION ALL
SELECT @event_id::uuid AS event_id, iso2, 'nearby' AS relation, round((meters / 1000)::numeric, 1)::real AS distance_km FROM nearby;

-- name: DeleteEvents :execrows
DELETE FROM events
WHERE source_id = @source_id
  AND source_event_id = ANY(@source_event_ids::text[]);

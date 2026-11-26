-- name: UpsertEvent :one
-- Inserts or updates one event and returns its id only when something
-- changed, so unchanged polls cost no writes and no country reassignment.
-- Geometry arrives as GeoJSON; Z (USGS depth) is dropped, depth lives in attrs.
INSERT INTO events (
    source_id, source_event_id, type, kind, status, severity, source_severity,
    title, summary, geom, centroid, started_at, observed_at, updated_at, ended_at,
    fetched_at, url, attrs, change_op, first_seen_at
) SELECT
    @source_id, @source_event_id, @type, @kind, @status, @severity, @source_severity,
    @title, @summary, r.g::geography,
    COALESCE(ST_SetSRID(ST_MakePoint(sqlc.narg('position_lon')::float8, sqlc.narg('position_lat')::float8), 4326),
             ST_PointOnSurface(r.g))::geography,
    @started_at, @observed_at, @updated_at, @ended_at,
    now(), @url, @attrs,
    CASE WHEN @status::text = 'resolved' THEN 'resolved' ELSE 'created' END,
    -- First seen active: now, or, for a new version of warnings it
    -- supersedes (MeteoAlarm), when the earliest of them was first seen,
    -- so an update is not reported as a new warning. Never set on an
    -- event that arrives already resolved, or on conflict (kept as is).
    CASE WHEN @status::text = 'resolved' THEN NULL
         WHEN EXISTS (SELECT 1 FROM events p WHERE p.source_id = @source_id
                      AND p.source_event_id = ANY(@supersedes::text[]))
         THEN (SELECT min(p.first_seen_at) FROM events p WHERE p.source_id = @source_id
               AND p.source_event_id = ANY(@supersedes::text[]))
         ELSE now() END
FROM (
    -- Feeds send the odd self-intersecting polygon; repair it here so
    -- country assignment and the map never trip over it. The structure
    -- method keeps an area an area (the default turned a dense coastline
    -- into an area plus stray lines, a GeometryCollection). It empties a
    -- shape that has collapsed entirely (a ring with no area, a line of
    -- one point); the default repair keeps that as a point or line.
    SELECT CASE WHEN ST_IsEmpty(v.s) THEN ST_MakeValid(v.raw) ELSE v.s END AS g
    FROM (
        SELECT x.raw, ST_MakeValid(x.raw, 'method=structure keepcollapsed=false') AS s
        FROM (SELECT ST_Force2D(ST_GeomFromGeoJSON(@geom::text)) AS raw) x
    ) v
) r
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
    attrs           = EXCLUDED.attrs,
    -- Stream cursor (migration 00005): only a real change moves it.
    change_seq      = nextval('event_change_seq'),
    change_op       = CASE WHEN EXCLUDED.status = 'resolved' AND events.status <> 'resolved'
                           THEN 'resolved' ELSE 'updated' END
WHERE (events.type, events.kind, events.status, events.severity, events.source_severity,
       events.title, events.summary, events.started_at, events.observed_at,
       events.updated_at, events.ended_at, events.url, events.attrs)
      IS DISTINCT FROM
      (EXCLUDED.type, EXCLUDED.kind, EXCLUDED.status, EXCLUDED.severity, EXCLUDED.source_severity,
       EXCLUDED.title, EXCLUDED.summary, EXCLUDED.started_at, EXCLUDED.observed_at,
       EXCLUDED.updated_at, EXCLUDED.ended_at, EXCLUDED.url, EXCLUDED.attrs)
   OR NOT ST_Equals(events.geom::geometry, EXCLUDED.geom::geometry)
   -- A source that pins its events (ADR 32): a move alone is a change.
   OR (sqlc.narg('position_lon')::float8 IS NOT NULL
       AND NOT ST_Equals(events.centroid::geometry, EXCLUDED.centroid::geometry))
RETURNING id;

-- name: AssignListedEventCountries :exec
-- The countries a source names for an event (CountryRule.Listed), instead
-- of those its geometry touches.
INSERT INTO event_countries (event_id, iso2, relation, distance_km)
SELECT @event_id::uuid, c.iso2, 'within', 0
FROM countries c WHERE c.iso2 = ANY(@iso2s::text[]);

-- name: ClearEventCountries :exec
DELETE FROM event_countries WHERE event_id = @event_id;

-- name: AssignEventCountries :exec
-- ADR 4: every country the geometry intersects ('within'); when there is
-- none, the nearest country within 250 km ('nearby').
--
-- A source whose areas a national service drew (MeteoAlarm) narrows that
-- with a rule: the issuer is always assigned, and another country only
-- when it holds at least min_share of the event's area. Their borders are
-- finer than Natural Earth's 1:50m ones, so plain intersection tagged
-- hundreds of warnings with neighbors they never covered.
--
-- Intersection is planar (geodesic took up to 32 s on one detailed area,
-- planar ~50 ms) except for a shape wider than 180° of longitude: that is
-- one stored across the antimeridian, which planar would wrap the long
-- way round.
WITH e AS (
    SELECT geom AS geog, geom::geometry AS g, ST_Area(geom) AS m2,
           ST_XMax(geom::geometry) - ST_XMin(geom::geometry) > 180 AS wraps
    FROM events WHERE id = @event_id
), touching AS (
    SELECT c.iso2, c.geom AS cgeom
    FROM countries c, e
    WHERE CASE WHEN e.wraps THEN ST_Intersects(c.geom, e.geog)
               ELSE ST_Intersects(c.geom::geometry, e.g) END
), within AS (
    SELECT t.iso2
    FROM touching t, e
    WHERE @min_share::float8 <= 0
       OR e.m2 = 0
       OR e.wraps
       OR t.iso2 = @issuer::text
       OR ST_Area(ST_Intersection(t.cgeom::geometry, e.g)::geography) >= e.m2 * @min_share::float8
    UNION
    SELECT c.iso2 FROM countries c WHERE c.iso2 = @issuer::text
), nearby AS (
    SELECT c.iso2, ST_Distance(c.geom, e.geom) AS meters
    FROM countries c, events e
    WHERE e.id = @event_id
      AND NOT EXISTS (SELECT 1 FROM within)
      AND ST_DWithin(c.geom, e.geom, 250000)
    ORDER BY c.geom <-> e.geom
    LIMIT 1
)
INSERT INTO event_countries (event_id, iso2, relation, distance_km)
SELECT @event_id::uuid AS event_id, iso2, 'within' AS relation, 0::real AS distance_km FROM within
UNION ALL
SELECT @event_id::uuid AS event_id, iso2, 'nearby' AS relation, round((meters / 1000)::numeric, 1)::real AS distance_km FROM nearby;

-- name: ActiveEventIDsWithAttr :many
-- A source's active events whose attrs[key] is value; MeteoAlarm uses it
-- to find what a country's full-snapshot feed no longer lists.
SELECT source_event_id FROM events
WHERE source_id = @source_id AND status = 'active' AND attrs ->> @attr_key::text = @attr_value::text;

-- name: ActiveEventIDs :many
-- A source's active events, for a source whose feed is a full window
-- (USGS's past day): what the window no longer lists has ended.
SELECT source_event_id FROM events
WHERE source_id = @source_id AND status = 'active';

-- name: ActiveEventStarts :many
-- A source's active events and when each started, for a source whose
-- messages do not carry the start once the event is under way (NWS).
SELECT source_event_id, started_at FROM events
WHERE source_id = @source_id AND status = 'active'
ORDER BY source_event_id;

-- name: ExpiredActiveEventIDs :many
-- A source's active events whose attrs.expiresAt (RFC 3339) has passed.
SELECT source_event_id FROM events
WHERE source_id = @source_id AND status = 'active'
  AND attrs ? 'expiresAt' AND (attrs ->> 'expiresAt')::timestamptz < @now::timestamptz;

-- name: ResolveEvent :execrows
-- Ends one active event its source no longer lists (owner, 2026-11-19).
-- updated_at stays the source's own (ADR 8); ended_at records when Shield
-- saw it go. Moves the stream cursor like any other change.
UPDATE events SET
    status     = 'resolved',
    ended_at   = coalesce(ended_at, @ended_at),
    fetched_at = now(),
    change_seq = nextval('event_change_seq'),
    change_op  = 'resolved'
WHERE source_id = @source_id AND source_event_id = @source_event_id AND status = 'active';

-- name: DeleteEvents :many
DELETE FROM events
WHERE source_id = @source_id
  AND source_event_id = ANY(@source_event_ids::text[])
RETURNING id;

-- name: InsertEventTombstones :exec
-- One tombstone per removed event, on the change sequence, so stream
-- clients learn about the removal.
INSERT INTO event_tombstones (event_id)
SELECT unnest(@event_ids::uuid[]);

-- name: LockEventChanges :exec
-- Serializes event writers until commit, so change_seq order is commit
-- order (migration 00005). The key is arbitrary but fixed.
SELECT pg_advisory_xact_lock(7268329100);

-- name: NotifyEventChanges :exec
-- Delivered at commit to every API pod LISTENing; the payload is empty
-- because listeners read the changes by cursor.
SELECT pg_notify('shield_events', '');

-- name: LatestEventChange :one
-- The highest committed change cursor; 0 when nothing has changed yet. The
-- stream floor keeps it from moving back when pruned tombstones held it.
SELECT GREATEST(
    COALESCE((SELECT max(change_seq) FROM events), 0),
    COALESCE((SELECT max(seq) FROM event_tombstones), 0),
    (SELECT seq FROM event_stream_floor)
)::bigint;

-- name: ExpiredResolvedEventIDs :many
-- Resolved events that ended before @before, oldest first (retention).
SELECT id FROM events
WHERE status = 'resolved' AND coalesce(ended_at, updated_at) < @before::timestamptz
ORDER BY coalesce(ended_at, updated_at)
LIMIT @max_rows;

-- name: DeleteEventsByID :many
DELETE FROM events WHERE id = ANY(@ids::uuid[]) RETURNING id;

-- name: PruneEventTombstones :one
-- Deletes tombstones written before @before and returns the highest seq
-- removed (0 when none).
WITH gone AS (
    DELETE FROM event_tombstones WHERE removed_at < @before::timestamptz RETURNING seq
)
SELECT COALESCE(max(seq), 0)::bigint AS max_seq, count(*)::bigint AS removed FROM gone;

-- name: RaiseStreamFloor :exec
UPDATE event_stream_floor SET seq = GREATEST(seq, @seq::bigint);

-- name: StreamFloor :one
SELECT seq FROM event_stream_floor;

-- name: EventSnapshot :one
-- The fields a revision tracks (ADR 6), locked for the rest of the upsert
-- transaction. The geometry is compared by hash, not stored.
SELECT id, severity, status, source_severity, title, ended_at,
       md5(ST_AsBinary(geom)) AS geom_hash
FROM events
WHERE source_id = @source_id AND source_event_id = @source_event_id
FOR UPDATE;

-- name: InsertEventRevision :exec
INSERT INTO event_revisions (event_id, changes) VALUES (@event_id, @changes);

-- name: ListEventRevisions :many
SELECT at, changes FROM event_revisions
WHERE event_id = @event_id
ORDER BY at DESC, id DESC
LIMIT @max_rows;

-- name: Superseded :one
-- The warnings a new version supersedes, to record the change across
-- versions as a revision of the new one: their highest severity, active
-- or ended ('' when none is stored), and whether any is still active.
SELECT COALESCE((array_agg(severity ORDER BY array_position(ARRAY['minor','moderate','severe','extreme'], severity) DESC NULLS LAST,
                                             updated_at DESC, id))[1], '')::text AS severity,
       COALESCE(bool_or(status = 'active'), false)::bool AS any_active
FROM events
WHERE source_id = @source_id AND source_event_id = ANY(@supersedes::text[]);

-- name: AdoptSupersededRevisions :exec
-- Moves the revisions of the warnings a new version supersedes onto it
-- before they are deleted, so a chain of versions keeps one history: the
-- severity at the window's start survives v1 -> v2 -> v3.
UPDATE event_revisions SET event_id = @event_id
WHERE event_id IN (SELECT p.id FROM events p WHERE p.source_id = @source_id
                   AND p.source_event_id = ANY(@supersedes::text[]) AND p.id <> @event_id);

-- name: KnownQuakes :many
-- Of the given quakes (parallel arrays), the 1-based positions of those
-- another source already stores, active or ended, within @seconds of the
-- origin time, @meters of the epicenter and @mag_delta of the magnitude:
-- the same earthquake reported by two networks (ADR 34).
SELECT q.idx::int AS idx
FROM unnest(@times::timestamptz[]) WITH ORDINALITY AS q(t, idx)
WHERE EXISTS (
    SELECT 1 FROM events e
    WHERE e.source_id = @source_id AND e.type = 'earthquake'
      AND e.observed_at BETWEEN q.t - make_interval(secs => @seconds::float8)
                            AND q.t + make_interval(secs => @seconds::float8)
      AND ST_DWithin(e.centroid,
                     ST_SetSRID(ST_MakePoint((@lons::float8[])[q.idx], (@lats::float8[])[q.idx]), 4326)::geography,
                     @meters::float8)
      -- A row whose mag is not a number matches nothing, rather than
      -- failing the cast and the poll.
      AND CASE WHEN jsonb_typeof(e.attrs -> 'mag') = 'number'
               THEN abs((e.attrs ->> 'mag')::float8 - (@mags::float8[])[q.idx]) <= @mag_delta::float8
               ELSE false END
)
ORDER BY q.idx;

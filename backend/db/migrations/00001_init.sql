-- Shield's initial schema. See docs/ARCHITECTURE.md §3 and docs/DECISIONS.md.
--
-- On the cluster, PostGIS is created by the Postgres initdb script as the
-- superuser; IF NOT EXISTS makes this a no-op there and creates it locally
-- and in CI, where the migrating role owns the database.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

-- One row per feed. Rows are written by the worker from the adapters' own
-- metadata, so adding a source is a code change, not a migration.
CREATE TABLE sources (
    id                    text PRIMARY KEY, -- 'usgs', 'gdacs', 'eonet', 'firms', 'meteoalarm'
    name                  text        NOT NULL,
    url                   text        NOT NULL,
    license               text        NOT NULL,
    attribution           text        NOT NULL,
    poll_interval_seconds integer     NOT NULL CHECK (poll_interval_seconds >= 30),
    enabled               boolean     NOT NULL DEFAULT true,
    -- HTTP validators from the last successful poll (conditional GET).
    etag                  text,
    last_modified         text,
    last_success_at       timestamptz,
    last_error            text,
    last_error_at         timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

-- Natural Earth admin-0 boundaries (public domain), loaded by
-- `shield seed-countries`.
CREATE TABLE countries (
    iso2 char(2) PRIMARY KEY,
    iso3 char(3)                          NOT NULL UNIQUE,
    name text                             NOT NULL,
    geom geography(MultiPolygon, 4326)    NOT NULL
);
CREATE INDEX countries_geom_idx ON countries USING gist (geom);

-- Every hazard, from every source, in one table (ADR 1). Hazard-specific
-- values (magnitude, depth, FRP, wind, PAGER level) live in attrs.
CREATE TABLE events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id       text        NOT NULL REFERENCES sources (id),
    source_event_id text        NOT NULL,
    type            text        NOT NULL CHECK (type IN (
                        'earthquake', 'wildfire', 'flood', 'storm', 'tropical_cyclone',
                        'volcano', 'drought', 'heat', 'cold', 'heavy_rain', 'wind',
                        'snow_ice', 'tsunami', 'other')),
    -- ADR 3: what kind of statement this record is.
    kind            text        NOT NULL CHECK (kind IN ('observed', 'warning', 'forecast', 'detection')),
    status          text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'resolved')),
    -- ADR 2: derived, CAP-aligned severity next to the source's own value.
    severity        text        NOT NULL CHECK (severity IN ('minor', 'moderate', 'severe', 'extreme')),
    source_severity text,
    title           text        NOT NULL,
    summary         text,
    geom            geography(Geometry, 4326) NOT NULL,
    centroid        geography(Point, 4326)    NOT NULL,
    started_at      timestamptz,
    observed_at     timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    ended_at        timestamptz,
    fetched_at      timestamptz NOT NULL DEFAULT now(),
    url             text,
    attrs           jsonb       NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (source_id, source_event_id)
);
CREATE INDEX events_geom_idx ON events USING gist (geom);
CREATE INDEX events_filter_idx ON events (status, type, severity, updated_at DESC);
CREATE INDEX events_updated_idx ON events (updated_at DESC);

-- ADR 4: countries come from ST_Intersects, not from source strings.
CREATE TABLE event_countries (
    event_id uuid    NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    iso2     char(2) NOT NULL REFERENCES countries (iso2),
    PRIMARY KEY (event_id, iso2)
);
CREATE INDEX event_countries_iso2_idx ON event_countries (iso2);

-- ADR 6: one row per meaningful change (severity, geometry, status).
CREATE TABLE event_revisions (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id uuid        NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    at       timestamptz NOT NULL DEFAULT now(),
    changes  jsonb       NOT NULL
);
CREATE INDEX event_revisions_event_idx ON event_revisions (event_id, at DESC);
CREATE INDEX event_revisions_at_idx ON event_revisions (at DESC);

-- ADR 10: FIRMS satellite hotspots, a 48h rolling window, never "wildfires".
-- lat/lon are the COPY target; point is derived for spatial queries.
CREATE TABLE fire_detections (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    satellite  text             NOT NULL,
    acq_at     timestamptz      NOT NULL,
    lat        double precision NOT NULL,
    lon        double precision NOT NULL,
    point      geography(Point, 4326) GENERATED ALWAYS AS
                   (ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography) STORED,
    frp        real,
    confidence text,
    iso2       char(2),
    UNIQUE (satellite, acq_at, lat, lon)
);
CREATE INDEX fire_detections_point_idx ON fire_detections USING gist (point);
CREATE INDEX fire_detections_acq_idx ON fire_detections (acq_at);

-- ADR 7: which sources cover which countries, for the coverage notice.
CREATE TABLE source_coverage (
    source_id text    NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    iso2      char(2) NOT NULL REFERENCES countries (iso2),
    PRIMARY KEY (source_id, iso2)
);

-- +goose Down
DROP TABLE source_coverage;
DROP TABLE fire_detections;
DROP TABLE event_revisions;
DROP TABLE event_countries;
DROP TABLE events;
DROP TABLE countries;
DROP TABLE sources;

-- MeteoAlarm warning areas (S1.5). Many feeds name a warning's areas by
-- code only, so the worker draws them from MeteoAlarm's own geocodes file
-- (CC BY 4.0): one polygon per EMMA_ID, plus the aliases some members use
-- instead (NUTS2, NUTS3, FIPS → EMMA_ID).
--
-- geo_datasets records which pinned file each table was loaded from, and
-- how it was processed ("<sha256>+<processing>"), so `shield migrate`
-- reloads it when either changes, not only when empty.

-- +goose Up
CREATE TABLE geo_datasets (
    name      text PRIMARY KEY,
    version   text NOT NULL,
    loaded_at timestamptz NOT NULL DEFAULT now()
);

-- Planar geometry: the resolver unions areas, and events store the result
-- as geography. EMMA_IDs look like DE411 or UK249.
CREATE TABLE meteoalarm_areas (
    code    text PRIMARY KEY CHECK (code ~ '^[A-Z]{2}[A-Z0-9]{1,6}$'),
    country text NOT NULL CHECK (country ~ '^[A-Z]{2}$'),
    name    text NOT NULL,
    geom    geometry(MultiPolygon, 4326) NOT NULL
);

-- One alias can stand for several EMMA_IDs (a NUTS2 region spans several
-- warning areas), and one EMMA_ID can have several aliases.
CREATE TABLE meteoalarm_area_aliases (
    alias_type text NOT NULL,
    alias_code text NOT NULL,
    code       text NOT NULL REFERENCES meteoalarm_areas (code) ON DELETE CASCADE,
    PRIMARY KEY (alias_type, alias_code, code)
);

-- +goose Down
DROP TABLE meteoalarm_area_aliases;
DROP TABLE meteoalarm_areas;
DROP TABLE geo_datasets;

-- NWS public forecast zones. Tsunami warnings (and other NWS products)
-- name the coasts they cover by UGC zone code (HIZ001, GUZ001) and carry no
-- shapes, so the worker draws them from NOAA's zone shapefile (public
-- domain), loaded like MeteoAlarm areas and recorded in geo_datasets.

-- +goose Up
CREATE TABLE nws_zones (
    ugc   text PRIMARY KEY CHECK (ugc ~ '^[A-Z]{2}Z[0-9]{3}$'),
    state text NOT NULL CHECK (state ~ '^[A-Z]{2}$'),
    name  text NOT NULL,
    geom  geometry(MultiPolygon, 4326) NOT NULL
);

-- +goose Down
DROP TABLE nws_zones;

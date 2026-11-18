-- Map queries filter by a lon/lat viewport rectangle, which is planar, so
-- they test e.geom::geometry. The geography index cannot serve that cast;
-- this expression index does.

-- +goose Up
CREATE INDEX events_geom_box_idx ON events USING gist ((geom::geometry));

-- +goose Down
DROP INDEX events_geom_box_idx;

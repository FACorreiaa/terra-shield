-- +goose Up
-- A source the API fetches on demand while someone watches (adsb.lol,
-- ADR 28) rather than one the worker polls: it is never stale, since an
-- unwatched layer is not a failing one.
ALTER TABLE sources ADD COLUMN on_demand boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE sources DROP COLUMN on_demand;

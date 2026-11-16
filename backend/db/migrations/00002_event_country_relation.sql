-- ADR 4 offshore fallback: an event that intersects no country is linked to
-- the nearest one within 250 km as 'nearby', with the distance, so the UI can
-- say "off the coast of ...".

-- +goose Up
ALTER TABLE event_countries
    ADD COLUMN relation    text NOT NULL DEFAULT 'within' CHECK (relation IN ('within', 'nearby')),
    ADD COLUMN distance_km real NOT NULL DEFAULT 0 CHECK (distance_km >= 0);

-- +goose Down
ALTER TABLE event_countries
    DROP COLUMN distance_km,
    DROP COLUMN relation;

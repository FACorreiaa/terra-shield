-- +goose Up
-- EMSC asks every two minutes which of its quakes USGS already stores
-- (ADR 34): by source and origin time, earthquakes only.
CREATE INDEX events_quake_time_idx ON events (source_id, observed_at) WHERE type = 'earthquake';

-- +goose Down
DROP INDEX events_quake_time_idx;

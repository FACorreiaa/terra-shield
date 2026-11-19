-- Retention (ADR 26): resolved events are deleted 90 days after they end,
-- and tombstones 7 days after they are written.
--
-- Pruning tombstones makes old stream cursors unresumable: a client whose
-- Last-Event-ID is below the newest pruned tombstone could miss a removal.
-- event_stream_floor holds that seq; a resume below it gets `reset` and
-- reloads. It also keeps the latest cursor from moving backwards when the
-- newest change was a pruned tombstone.

-- +goose Up
CREATE TABLE event_stream_floor (
    singleton boolean PRIMARY KEY DEFAULT true CONSTRAINT event_stream_floor_one_row CHECK (singleton),
    seq       bigint  NOT NULL
);
INSERT INTO event_stream_floor (seq) VALUES (0);

-- The purge's scan: resolved events by when they ended.
CREATE INDEX events_resolved_end_idx ON events ((coalesce(ended_at, updated_at))) WHERE status = 'resolved';

-- +goose Down
DROP INDEX events_resolved_end_idx;
DROP TABLE event_stream_floor;

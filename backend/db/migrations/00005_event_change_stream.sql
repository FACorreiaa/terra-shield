-- Change cursor for the live stream (S1.7, S2.6).
--
-- Every insert or real change of an event takes the next value of
-- event_change_seq into events.change_seq, and change_op says what kind of
-- change it was. A removed event leaves a tombstone on the same sequence.
-- A stream client's Last-Event-ID is a value of this sequence: everything
-- after it is "events with a higher change_seq, plus tombstones with a
-- higher seq". Writers hold an advisory transaction lock (store), so
-- sequence order is commit order and a reader never skips an in-flight
-- write.
--
-- Tombstones are kept: event deletions are rare (USGS retractions), and
-- keeping them makes any cursor resumable. A future event-retention purge
-- must write tombstones too.

-- +goose Up
CREATE SEQUENCE event_change_seq;

ALTER TABLE events
    ADD COLUMN change_seq bigint NOT NULL DEFAULT nextval('event_change_seq'),
    ADD COLUMN change_op  text   NOT NULL DEFAULT 'created'
        CONSTRAINT events_change_op_check CHECK (change_op IN ('created', 'updated', 'resolved'));

ALTER SEQUENCE event_change_seq OWNED BY events.change_seq;

CREATE UNIQUE INDEX events_change_seq_idx ON events (change_seq);

CREATE TABLE event_tombstones (
    seq        bigint      PRIMARY KEY DEFAULT nextval('event_change_seq'),
    event_id   uuid        NOT NULL,
    removed_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE event_tombstones;
ALTER TABLE events DROP COLUMN change_op, DROP COLUMN change_seq;

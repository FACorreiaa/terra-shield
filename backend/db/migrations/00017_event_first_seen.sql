-- "What changed in 24h" (BACKLOG queue item 2) needs to know when Shield
-- first saw an event active: revisions record changes to an event, not its
-- arrival, and change_op is overwritten by the next update. Set once, on
-- insert. Rows stored before this migration stay NULL, so they are never
-- reported as new (a DEFAULT now() would mark every stored event new).

-- +goose Up
ALTER TABLE events ADD COLUMN first_seen_at timestamptz;
CREATE INDEX events_first_seen_idx ON events (first_seen_at) WHERE first_seen_at IS NOT NULL;

-- +goose Down
DROP INDEX events_first_seen_idx;
ALTER TABLE events DROP COLUMN first_seen_at;

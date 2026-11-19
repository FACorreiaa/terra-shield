-- +goose Up
-- What the Sources screen says about each source (S3.7): one line on what
-- it covers, its terms page, and how often it publishes. Written by the
-- worker from each adapter's Info(); NULL until it registers.
ALTER TABLE sources
    ADD COLUMN scope text,
    ADD COLUMN terms_url text,
    ADD COLUMN cadence text;

-- +goose Down
ALTER TABLE sources
    DROP COLUMN scope,
    DROP COLUMN terms_url,
    DROP COLUMN cadence;

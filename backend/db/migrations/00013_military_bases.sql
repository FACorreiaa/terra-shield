-- +goose Up
-- Military installations listed on Wikidata (L3, ADR 29): a static layer,
-- not events. Replaced wholesale on each weekly poll. Points only, so plain
-- lat/lon columns and a range test, like fire_detections.
CREATE TABLE military_bases (
    qid          text PRIMARY KEY,
    name         text             NOT NULL,
    description  text,
    kind         text             NOT NULL,
    class        text             NOT NULL,
    operator     text,
    lat          double precision NOT NULL,
    lon          double precision NOT NULL,
    -- From our own country geometry, else Wikidata's country (P17).
    iso2         text,
    country_hint text,
    updated_at   timestamptz      NOT NULL
);
CREATE INDEX military_bases_lat ON military_bases (lat);

-- +goose Down
DROP TABLE military_bases;

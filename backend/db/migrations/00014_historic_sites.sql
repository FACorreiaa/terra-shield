-- +goose Up
-- The historic layer (L4, ADR 30): forts, castles, bunkers, war cemeteries
-- and other historic sites listed on Wikidata. Static, replaced on each
-- weekly poll. Points only, so plain lat/lon columns, like military_bases.
-- Items that are on military_bases (in use) are left out when reading.
CREATE TABLE historic_sites (
    qid          text PRIMARY KEY,
    name         text             NOT NULL,
    description  text,
    kind         text             NOT NULL,
    class        text             NOT NULL,
    lat          double precision NOT NULL,
    lon          double precision NOT NULL,
    iso2         text,
    country_hint text,
    updated_at   timestamptz      NOT NULL
);
CREATE INDEX historic_sites_lat ON historic_sites (lat);

-- +goose Down
DROP TABLE historic_sites;

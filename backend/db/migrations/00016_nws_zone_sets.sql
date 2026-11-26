-- NWS alerts name three kinds of area by UGC code: public forecast zones
-- (ILZ080), fire weather zones (WAZ658, numbered apart from the forecast
-- zones with the same code) and counties (NCC037, used by watches and
-- storm-based warnings). Each comes from its own NOAA shapefile (public
-- domain), loaded like the forecast zones, so the table gains the set a
-- zone belongs to and holds county codes too.

-- +goose Up
ALTER TABLE nws_zones DROP CONSTRAINT nws_zones_pkey;
ALTER TABLE nws_zones DROP CONSTRAINT nws_zones_ugc_check;
ALTER TABLE nws_zones ADD COLUMN zone_set text NOT NULL DEFAULT 'forecast'
    CHECK (zone_set IN ('forecast', 'fire', 'county'));
ALTER TABLE nws_zones ALTER COLUMN zone_set DROP DEFAULT;
ALTER TABLE nws_zones ADD CONSTRAINT nws_zones_ugc_check CHECK (ugc ~ '^[A-Z]{2}[ZC][0-9]{3}$');
ALTER TABLE nws_zones ADD PRIMARY KEY (zone_set, ugc);

-- +goose Down
DELETE FROM nws_zones WHERE zone_set <> 'forecast';
DELETE FROM geo_datasets WHERE name IN ('nws_fire_zones', 'nws_counties');
ALTER TABLE nws_zones DROP CONSTRAINT nws_zones_pkey;
ALTER TABLE nws_zones DROP CONSTRAINT nws_zones_ugc_check;
ALTER TABLE nws_zones DROP COLUMN zone_set;
ALTER TABLE nws_zones ADD CONSTRAINT nws_zones_ugc_check CHECK (ugc ~ '^[A-Z]{2}Z[0-9]{3}$');
ALTER TABLE nws_zones ADD PRIMARY KEY (ugc);

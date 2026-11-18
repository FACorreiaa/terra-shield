-- ADR 7 coverage transparency, and per-country event stats for the
-- countries API (S2.4).
--
-- sources.official_warnings marks feeds that carry official national
-- warnings (MeteoAlarm). A country covered by none of them gets a "limited
-- official-warning coverage" notice. source_coverage rows list a source's
-- countries; a source with no rows covers the whole world.

-- +goose Up
ALTER TABLE sources
    ADD COLUMN official_warnings boolean NOT NULL DEFAULT false;

-- Active events per country count offshore 'nearby' events too, matching
-- /v1/events?country=. The bbox is planar lon/lat, so a country that
-- crosses the antimeridian spans -180..180.
CREATE VIEW country_stats AS
SELECT c.iso2::text AS iso2,
       c.iso3::text AS iso3,
       c.name,
       count(e.id)::integer AS active_events,
       -- '' when the country has no active events
       COALESCE((ARRAY['minor', 'moderate', 'severe', 'extreme'])[
           max(array_position(ARRAY['minor', 'moderate', 'severe', 'extreme'], e.severity))
       ], '')::text AS max_severity,
       ST_XMin(c.geom::geometry)::double precision AS min_lon,
       ST_YMin(c.geom::geometry)::double precision AS min_lat,
       ST_XMax(c.geom::geometry)::double precision AS max_lon,
       ST_YMax(c.geom::geometry)::double precision AS max_lat
FROM countries c
LEFT JOIN event_countries ec ON ec.iso2 = c.iso2
LEFT JOIN events e ON e.id = ec.event_id AND e.status = 'active'
GROUP BY c.iso2;

-- +goose Down
DROP VIEW country_stats;
ALTER TABLE sources DROP COLUMN official_warnings;

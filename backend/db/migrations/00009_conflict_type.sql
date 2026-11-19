-- +goose Up
-- Armed conflicts become an event type (ADR 27). They are not hazards:
-- hazard-only surfaces (the country counts here, and the API's defaults)
-- leave them out unless asked for.
ALTER TABLE events DROP CONSTRAINT events_type_check;
ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN (
    'earthquake', 'wildfire', 'flood', 'storm', 'tropical_cyclone',
    'volcano', 'drought', 'heat', 'cold', 'heavy_rain', 'wind',
    'snow_ice', 'tsunami', 'other', 'conflict'));

CREATE OR REPLACE VIEW country_stats AS
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
LEFT JOIN events e ON e.id = ec.event_id AND e.status = 'active' AND e.type <> 'conflict'
GROUP BY c.iso2;

-- +goose Down
CREATE OR REPLACE VIEW country_stats AS
SELECT c.iso2::text AS iso2,
       c.iso3::text AS iso3,
       c.name,
       count(e.id)::integer AS active_events,
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
DELETE FROM events WHERE type = 'conflict';
ALTER TABLE events DROP CONSTRAINT events_type_check;
ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN (
    'earthquake', 'wildfire', 'flood', 'storm', 'tropical_cyclone',
    'volcano', 'drought', 'heat', 'cold', 'heavy_rain', 'wind',
    'snow_ice', 'tsunami', 'other'));

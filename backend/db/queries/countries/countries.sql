-- name: CountCountries :one
SELECT count(*) FROM countries;

-- name: ListCountryStats :many
SELECT * FROM country_stats ORDER BY name;

-- name: GetCountryStats :one
SELECT * FROM country_stats WHERE iso2 = @iso2::text;

-- name: CountrySeverityCounts :many
-- Active events in or near the country, by type and severity; most severe
-- first.
SELECT e.type, e.severity, count(*)::integer AS events
FROM events e
JOIN event_countries ec ON ec.event_id = e.id
WHERE ec.iso2 = @iso2::text AND e.status = 'active'
GROUP BY e.type, e.severity
ORDER BY array_position(ARRAY['minor', 'moderate', 'severe', 'extreme'], e.severity) DESC, events DESC, e.type;

-- name: CountryCoverage :many
-- Enabled sources that cover the country: worldwide ones (no coverage
-- rows) and those listing it.
SELECT s.id, s.official_warnings
FROM sources s
WHERE s.enabled
  AND (NOT EXISTS (SELECT 1 FROM source_coverage sc WHERE sc.source_id = s.id)
       OR EXISTS (SELECT 1 FROM source_coverage sc WHERE sc.source_id = s.id AND sc.iso2 = @iso2::text))
ORDER BY s.id;

-- name: CountryRecentChanges :many
SELECT r.event_id, e.title, r.at, r.changes
FROM event_revisions r
JOIN events e ON e.id = r.event_id
WHERE r.at >= @since
  AND EXISTS (SELECT 1 FROM event_countries ec WHERE ec.event_id = r.event_id AND ec.iso2 = @iso2::text)
ORDER BY r.at DESC, r.id DESC
LIMIT @max_rows;

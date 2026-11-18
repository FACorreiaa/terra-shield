-- name: ListSources :many
SELECT id, name, url, license, attribution, poll_interval_seconds, enabled,
       last_success_at, last_error, last_error_at
FROM sources
ORDER BY id;

-- name: UpsertSource :exec
INSERT INTO sources (id, name, url, license, attribution, poll_interval_seconds, enabled, official_warnings)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name                  = EXCLUDED.name,
    url                   = EXCLUDED.url,
    license               = EXCLUDED.license,
    attribution           = EXCLUDED.attribution,
    poll_interval_seconds = EXCLUDED.poll_interval_seconds,
    enabled               = EXCLUDED.enabled,
    official_warnings     = EXCLUDED.official_warnings,
    updated_at            = now();

-- name: ClearSourceCoverage :exec
DELETE FROM source_coverage WHERE source_id = @source_id;

-- name: AddSourceCoverage :execrows
-- Unknown codes (no such country loaded) are skipped, not an error.
INSERT INTO source_coverage (source_id, iso2)
SELECT @source_id, c.iso2 FROM countries c WHERE c.iso2 = ANY(@iso2_codes::text[])
ON CONFLICT DO NOTHING;

-- name: GetSourceValidators :one
SELECT etag, last_modified
FROM sources
WHERE id = $1;

-- name: RecordSourceSuccess :exec
UPDATE sources
SET last_success_at = $2,
    etag            = $3,
    last_modified   = $4,
    updated_at      = now()
WHERE id = $1;

-- name: RecordSourceError :exec
UPDATE sources
SET last_error    = $2,
    last_error_at = $3,
    updated_at    = now()
WHERE id = $1;

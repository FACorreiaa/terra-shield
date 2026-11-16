-- name: ListSources :many
SELECT id, name, url, license, attribution, poll_interval_seconds, enabled,
       last_success_at, last_error, last_error_at
FROM sources
ORDER BY id;

-- name: UpsertSource :exec
INSERT INTO sources (id, name, url, license, attribution, poll_interval_seconds, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    name                  = EXCLUDED.name,
    url                   = EXCLUDED.url,
    license               = EXCLUDED.license,
    attribution           = EXCLUDED.attribution,
    poll_interval_seconds = EXCLUDED.poll_interval_seconds,
    enabled               = EXCLUDED.enabled,
    updated_at            = now();

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

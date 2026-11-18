-- name: PurgeFireDetections :execrows
-- ADR 10: fire_detections is a rolling window.
DELETE FROM fire_detections WHERE acq_at < @before;

-- name: CountFireDetections :one
SELECT count(*) FROM fire_detections;

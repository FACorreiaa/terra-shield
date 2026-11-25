-- name: CountMilitaryBases :one
SELECT count(*) FROM military_bases;

-- name: MilitaryBasesInBox :many
-- min_lon greater than max_lon crosses the antimeridian.
SELECT qid, name, description, kind, class, operator, lat, lon, iso2, updated_at
FROM military_bases
WHERE lat BETWEEN @min_lat::float8 AND @max_lat::float8
  AND CASE WHEN @min_lon::float8 <= @max_lon::float8
           THEN lon BETWEEN @min_lon::float8 AND @max_lon::float8
           ELSE lon >= @min_lon::float8 OR lon <= @max_lon::float8
      END
ORDER BY qid;

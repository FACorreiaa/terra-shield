-- name: CountNWSZones :one
SELECT count(*) FROM nws_zones WHERE zone_set = @zone_set::text;

-- name: UnionNWSZones :one
-- The shape of the forecast zones a product names, dissolved into one
-- MultiPolygon in a fixed order so an unchanged warning yields the same
-- geometry every poll, and the codes we hold among those asked for.
-- 6 decimals ≈ 0.1 m.
SELECT COALESCE(ST_AsGeoJSON(ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_Union(geom ORDER BY ugc)), 3)), 6), '')::text AS geometry,
       COALESCE(array_agg(ugc ORDER BY ugc), '{}')::text[] AS found
FROM nws_zones WHERE zone_set = 'forecast' AND ugc = ANY(@ugcs::text[]);

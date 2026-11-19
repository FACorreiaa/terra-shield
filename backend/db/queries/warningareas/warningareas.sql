-- name: GetDatasetVersion :one
SELECT version FROM geo_datasets WHERE name = @name;

-- name: CountWarningAreas :one
SELECT count(*) FROM meteoalarm_areas;

-- name: WarningAreaCodes :many
-- The EMMA_IDs behind each requested geocode: the code itself when it is
-- an EMMA_ID we hold, else every EMMA_ID its alias stands for. A geocode
-- with no row here is unknown.
WITH req AS (
    SELECT t.type, v.value
    FROM unnest(@types::text[]) WITH ORDINALITY AS t(type, i)
    JOIN unnest(@values::text[]) WITH ORDINALITY AS v(value, i) ON v.i = t.i
)
SELECT req.type::text AS type, req.value::text AS value, a.code
FROM req JOIN meteoalarm_areas a ON req.type = 'EMMA_ID' AND a.code = req.value
UNION ALL
SELECT req.type::text, req.value::text, al.code
FROM req JOIN meteoalarm_area_aliases al ON al.alias_type = req.type AND al.alias_code = req.value;

-- name: UnionWarningAreas :one
-- One warning's shape: its areas and inline CAP polygons dissolved into a
-- MultiPolygon. Parts are unioned in a fixed order so an unchanged warning
-- yields the same geometry every poll (the event upsert compares shapes).
-- 6 decimals is about 0.1 m.
WITH parts AS (
    SELECT code AS k, geom FROM meteoalarm_areas WHERE code = ANY(@codes::text[])
    UNION ALL
    SELECT '~' || lpad(p.ord::text, 6, '0'), ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON(p.g), 4326))
    FROM unnest(@polygons::text[]) WITH ORDINALITY AS p(g, ord)
)
-- '' when nothing matched (the caller always sends at least one part).
SELECT COALESCE(ST_AsGeoJSON(ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_Union(geom ORDER BY k)), 3)), 6), '')::text AS geometry
FROM parts;

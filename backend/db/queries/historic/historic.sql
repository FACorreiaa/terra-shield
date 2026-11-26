-- name: CountHistoricSites :many
-- Per kind and class, for the shrink guard, with the World Heritage flags.
SELECT kind, class, count(*)::integer AS n,
       (count(*) FILTER (WHERE world_heritage))::integer AS world_heritage
FROM historic_sites GROUP BY kind, class;

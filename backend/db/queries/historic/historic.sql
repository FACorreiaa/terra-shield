-- name: CountHistoricSitesByKind :many
SELECT kind, count(*)::integer AS n FROM historic_sites GROUP BY kind;

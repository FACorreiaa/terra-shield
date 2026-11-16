# Shield — Design Decisions

## 2. Design decisions (→ `docs/adr/`)

1. **Unified event model.** One `events` table for every hazard. Hazard-specific fields (magnitude, depth, FRP, wind, PAGER) go in `attrs jsonb`.
2. **Two severities, never merged.** Store `source_severity` verbatim (GDACS green/orange/red, Meteoalarm level 2–4, USGS mag + PAGER). Derive `severity ∈ {minor, moderate, severe, extreme}` (CAP-aligned) in `internal/severity` from mapping tables per source. The tables are documented in `docs/severity.md` and covered by table tests. The UI shows both values.
3. **`kind` on every record:** `observed | warning | forecast | detection`. FIRMS hotspots are `detection`, never labelled "wildfire".
4. **Countries from geometry.** Natural Earth admin-0 1:50m (public domain) goes into `countries`. `event_countries` is built with `ST_Intersects`, so an event spanning PT+ES matches both. Codes sent by sources are only hints.
5. **Dedupe** by `(source_id, source_event_id)` upsert. Cross-source merging comes after the MVP; until then the UI shows source badges.
6. **Revisions.** `event_revisions` records severity, geometry and status changes. This makes a later "what changed in 24h" feature cheap.
7. **Coverage transparency.** `source_coverage(source_id, iso2)`. The country view shows a "limited official-warning coverage" notice. Absence of data ≠ safety.
8. **Freshness.** Each event carries `observed_at`, `updated_at` and `fetched_at`. `/v1/sources` exposes each source's last success. Clients grey out stale data.
9. **Map payload.** Bbox + zoom → GeoJSON. At low zoom, cluster on the server (grid snap / `ST_ClusterDBSCAN`) and simplify polygons per zoom. Use **vector tiles (`ST_AsMVT`) only when measured payloads hurt.**
10. **FIRMS is a separate layer.** `fire_detections` keeps a 48h rolling window, purged hourly by a River job. The node has 60 GB shared, so size is watched.
11. **Licensing gate.** MVP sources: GDACS, USGS, NASA EONET, NASA FIRMS (MAP_KEY, attribution), Meteoalarm (check per-member terms). **Open-Meteo / RainViewer are excluded** (non-commercial). Sources & attribution appear in the app.
12. **Safety copy.** A permanent disclaimer says Shield does not replace national emergency services. Every event links to its official source.
13. **No Redis, Kafka, object storage or tile server in the MVP.** Add any of them only on measured need.
14. **Tests never hit real feeds.** Recorded fixtures + `httptest` stubs. DB tests use norviq's `internal/testutil/testdb.go` pattern (`TEST_DATABASE_URL`, template DB cloned per test). CI service image is `postgis/postgis:18-3.6`.
15. **Low metric cardinality.** Grafana Cloud's free tier allows about 10k series. Use per-source labels only, never per-event or per-country.


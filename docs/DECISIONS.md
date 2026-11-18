# Shield — Design Decisions

## 2. Design decisions (→ `docs/adr/`)

1. **Unified event model.** One `events` table for every hazard. Hazard-specific fields (magnitude, depth, FRP, wind, PAGER) go in `attrs jsonb`.
2. **Two severities, never merged.** Store `source_severity` verbatim (GDACS green/orange/red, Meteoalarm level 2–4, USGS mag + PAGER). Derive `severity ∈ {minor, moderate, severe, extreme}` (CAP-aligned) in `internal/severity` from mapping tables per source. The tables are documented in `docs/severity.md` and covered by table tests. The UI shows both values.
3. **`kind` on every record:** `observed | warning | forecast | detection`. FIRMS hotspots are `detection`, never labelled "wildfire".
4. **Countries from geometry.** Natural Earth admin-0 1:50m v5.1.2 (public domain, SHA-256 pinned) goes into `countries`. `migrate` seeds it on an empty table, and `seed-countries` reloads it. `event_countries` is built with `ST_Intersects`, so an event spanning PT+ES matches both. Codes sent by sources are only hints.
   - **Codes follow ISO 3166-1.** Somaliland → SO and Northern Cyprus → CY; Kosovo is XK/XKX. Siachen Glacier has no ISO assignment, so it is skipped. Features sharing a code (Australia and its territories) are unioned.
   - **Offshore fallback** (measured 2026-11-16). At 1:50m, coasts and estuaries are coarse: the Lisbon point is 2.3 km outside PT, the Tōhoku epicentre 81 km off JP, Gorringe Bank 211 km off PT. When nothing intersects, assign the **nearest country within 250 km** with `relation = 'nearby'` and its distance. Intersections are `relation = 'within'`. Beyond 250 km an event has no country (e.g. mid-Atlantic ridge, 892 km). The UI says "off the coast of …" for `nearby`. This needs migration 00002 (`event_countries.relation`, `distance_km`) in S1.7.
5. **Dedupe** by `(source_id, source_event_id)` upsert. Cross-source merging comes after the MVP; until then the UI shows source badges. **Amended 2026-11-16:** where two sources carry the same events from the same origin, one source owns the type instead. GDACS takes its earthquakes from USGS/NEIC, so the GDACS adapter skips EQ, and USGS (with its PAGER impact alert) owns earthquakes. Without this, every large quake would show twice. EONET, an aggregator, drops anything from GDACS, plus earthquakes and JTWC cyclone tracks, for the same reason.
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
16. **Live by default: near real time wherever a source allows** (decided 2026-11-16). Shield is a "right now" product, so data age is a product feature, not an ops detail.
    - **Poll at each source's own update cadence, never slower.** Polling faster than a source publishes buys nothing, so the source sets the ceiling. Each source's published cadence and rate limits are verified and recorded in `docs/sources-licensing.md` (S0.2) before its interval is set. Starting points: USGS `2.5_day` every 60 s (the feed regenerates each minute; the day window carries revisions); GDACS and Meteoalarm every 5 min; EONET every 10 min; FIRMS every 15 min (within its MAP_KEY transaction limit; NRT detections arrive hours after the satellite pass, and the UI says so). Conditional GET keeps fast polling cheap for both sides.
    - **Prefer push over polling** wherever a source offers it (streaming feed, webhook, websocket). Re-check per source as sources evolve.
    - **Push end to end, no client polling.** Worker commit → `NOTIFY` → API `LISTEN` → SSE → client applies the change in place. The client polls only as a fallback when SSE is unavailable. On foreground, the app reconnects with `Last-Event-ID` and receives what it missed, without reloading the world.
    - **Latency budget.** Source publishes → visible on the map within **one poll interval + 5 s**. HTTP cache lifetimes stay below the shortest poll interval (`max-age` ≤ 15 s on event lists).
    - **Measure it.** `shield_source_lag_seconds` (our `fetched_at` minus the source's own update timestamp, per source) and `shield_event_publish_seconds` (commit → SSE fan-out). Alert when a source's lag exceeds 3× its interval, the same threshold as `stale` in `/v1/sources`.
    - **Honest freshness.** Every event shows "updated N min ago" and its source. Stale sources and events are visibly greyed (ADR 8). A source's slowness (FIRMS NRT, EONET curation) is labelled, never hidden.
    - **Limits.** Respect each source's terms and rate limits. On errors, back off with jitter and honour `Retry-After`. Being blocked by a source is the worst latency of all.
17. **Dependency injection with repositories** (decided 2026-11-16).
    - `internal/domain` holds the core types and imports nothing from the module.
    - `internal/store` has one repository per aggregate (events, sources, countries, fires). Stable queries are sqlc; bulk paths (COPY, staging tables) use pgx directly.
    - Consumers declare the narrow interface they need where they use it: `ingest.SourceStates`, `ingest.EventStore`, `ingest.FireStore`, `sources.Lister`, `server.Pinger`. No consumer imports `store`. Tests use in-memory fakes; repositories have their own tests against PostGIS.
    - `internal/app` is the only place that constructs dependencies: config, logger, pool, repositories, HTTP clients, adapters, features. `cmd/shield/main.go` only installs the signal context and calls `app.Run`.
    - Features are constructed with their dependencies and only mount routes; `server.Deps` carries just config, logger and health.
18. **API evolution: open `type`, closed everything else** (decided 2026-11-16, with the Apple client session).
    - Generated clients (swift-openapi-generator) decode string enums as closed types. One unknown value fails the whole `/v1/events` response in an app that has already shipped.
    - Event `type` will grow (landslide, avalanche, …), so the spec types it as `OpenEventType`: `anyOf [EventType, string]`. Clients map unknown values to `other`. Adding a type is not a breaking change.
    - `kind`, `status`, `severity` and country `relation` are fixed by ADRs 2–4 and stay closed enums. Adding a value to any of them is a breaking change: either `/v2`, or the server maps the new value to an existing one for older clients.
    - New optional fields and new `attrs` keys are never breaking. Clients must ignore unknown keys.
19. **Web client: React + TypeScript on Vite+, not SvelteKit** (decided by the owner 2026-11-16; merged as PR #1, `cb86131`).
    - Lives in `shield-app-web/`. Stack: TanStack Router + Query, Tailwind v4 with the generated design tokens, shadcn/ui, MapLibre GL JS 6 with a bundled Natural Earth basemap (no third-party tiles).
    - API client: `openapi-typescript` + `openapi-fetch` in `shield-app-web/packages/api-client`, generated from `backend/api/openapi.yaml`, like the Swift `shield-api` package.
    - Tooling: Vitest + Playwright; oxlint with every stable category; TS-Go type checking via `vp check`.
    - **Spec changes ship with both regenerated clients.** `ci-web.yml`'s `generated` job fails when `openapi.yaml` changes without `packages/api-client/src/schema.gen.ts` (regenerate with `bun run generate` in that package); `ci-apple.yml`'s `api` job does the same for Swift.
    - Hosting is still open: a static build behind Caddy, or embedded in the Go binary (Postitheka pattern).
    - Supersedes the SvelteKit plan in the original architecture.
20. **Live stream: a change cursor in Postgres, NOTIFY as a doorbell** (decided 2026-11-18, S1.7 + S2.6).
    - Every insert or real change of an event takes the next `event_change_seq` value into `events.change_seq`, with `change_op` (created, updated, resolved). A deletion leaves a row in `event_tombstones` on the same sequence. Tombstones are kept: deletions are rare, and keeping them makes every cursor resumable.
    - Event writers take `pg_advisory_xact_lock` before writing, so sequence order is commit order. Without it, a reader could pass cursor 11 while a slower transaction still held 10, and miss it for good.
    - `NOTIFY shield_events` carries no payload; it only wakes readers. Each API pod's hub reads "everything after my cursor" once per wake (plus a 30 s safety poll) and fans out. A lost notification, a reconnecting listener or a new pod all recover by reading the cursor, and nothing depends on NOTIFY's 8 KB payload limit or delivery.
    - Clients resume with `Last-Event-ID` and receive the current state of each event changed since, coalesced. More than 1,000 changes means `reset` and a reload from `/v1/events`.
    - A slow client is dropped, not waited for. It reconnects and catches up by cursor, so one bad connection never delays the others.
    - Rejected: an outbox table (one more table to purge, for nothing the cursor does not do); replaying `event_revisions` (it records only user-visible field changes, not creations or removals); payload-carrying NOTIFY (8 KB limit, lost when no one listens).


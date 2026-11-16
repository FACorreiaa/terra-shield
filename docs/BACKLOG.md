# Shield — MVP Backlog

## 8. MVP backlog

### M0 — Foundations
- [x] S0.1 Step 0 housekeeping (CLAUDE.md, memory)
- [~] S0.2 `git init` ✓, GitHub `FACorreiaa/terra-shield` ✓, README ✓, decisions doc ✓, `.gitignore` + `backend/.dockerignore` ✓ — `docs/sources-licensing.md` (terms verified per source) pending
- [x] S0.3 Backend skeleton: Echo v5, config, slog, shutdown, `/health/live` `/health/ready` `/metrics` `/openapi.yaml`, `routes` subcommand, routes ⊆ spec test, `/v1/sources`
- [x] S0.4 Goose + River migrations, schema 00001, `migrate` subcommand
- [x] S0.5 sqlc setup + `testutil` template-DB harness
- [x] S0.6 `seed-countries` + auto-seed in `migrate` (Natural Earth 1:50m v5.1.2, SHA-256 pinned, ISO 3166 overrides; 237 countries, all valid)
- [x] S0.7 Dockerfile, Makefile, `ci-backend.yml` (PostGIS service, `sqlc diff`, `-race`, golangci-lint v2). `image.yml` moved to S3.9a.
- S0.8 Infra → **moved to S3.9a**. Development is local until then (see "Local-first" below).

### M1 — Ingestion
- [~] S1.1 `Source` interface + River periodic harness (unique per source) ✓ — `internal/fetch` (adapted from feeds) pending
- [ ] S1.2 USGS GeoJSON (`all_hour` every 1 min, `all_day` reconcile every 15 min)
- [ ] S1.3 GDACS events API (EQ/TC/FL/VO/WF/DR, alert levels, polygons) every 5 min (ADR 16)
- [ ] S1.4 NASA EONET v3 (open events) every 10 min
- [ ] S1.5 Meteoalarm CAP warnings (EU) every 5 min
- [ ] S1.6 NASA FIRMS VIIRS NRT (global 24h CSV) every 15 min (within MAP_KEY limits) → `CopyFrom` → `fire_detections`; hourly purge > 48h
- [ ] S1.7 Normalizer: `ST_Intersects` country assignment + 250 km `nearby` offshore fallback (migration 00002: `event_countries.relation`, `distance_km`), revision diff, `NOTIFY`
- [ ] S1.8 `internal/severity` tables + tests + `docs/severity.md`
- [ ] S1.9 Source health in DB + metrics + staleness alert + freshness metrics `shield_source_lag_seconds`, `shield_event_publish_seconds` (ADR 16)
- [ ] S1.10 Fixtures + stubs for every adapter

### M2 — API
- [ ] S2.1 `openapi.yaml` v1 + routes ⊆ spec test
- [ ] S2.2 `/v1/events`: filters, bbox, zoom clustering, GeoJSON negotiation, ETag, `max-age` ≤ 15 s
- [ ] S2.3 `/v1/events/{id}` with revisions
- [ ] S2.4 `/v1/countries` + summary with coverage
- [ ] S2.5 `/v1/fires`, `/v1/sources`
- [ ] S2.6 `/v1/stream` SSE (LISTEN hub, heartbeat, filters, `Last-Event-ID` replay of missed changes from `event_revisions`, closed on shutdown); budget: source publish → client ≤ 1 poll interval + 5 s
- [ ] S2.7 Rate limit, CORS, gzip; budget: world view < 300 KB gzipped, p95 < 200 ms

### M3 — Apple app
- [ ] S3.1 Xcode project (Aaru pattern), `shield-api` + `shield-core` packages, `generate-api-client.sh`, tokens pipeline
- [ ] S3.2 Map: MKMapView clustering, severity colour + SF Symbol per type, FIRMS layer toggle
- [ ] S3.3 Filter sheet (type, severity, kind, time window), persisted locally
- [ ] S3.4 Country picker → summary (counts, coverage notice, list)
- [ ] S3.5 Event detail: source badge, source vs derived severity, freshness, timeline, "Open official source"
- [ ] S3.6 Live updates via SSE (`URLSession.bytes`): changes applied in place, reconnect with `Last-Event-ID` on foreground, polling only as fallback; "updated N min ago" on every event
- [ ] S3.7 Sources & attribution screen + safety disclaimer
- [ ] S3.8 macOS layout: `NavigationSplitView` sidebar + map, keyboard shortcuts
- [ ] S3.9a **Go-live of the API** (only once the TestFlight build is ready, or the day before MVP): pick the hostname, Cloudflare DNS A record, `image.yml` (ghcr push + promote PR into LuminaVaultInfra, `INFRA_TOKEN`), and in `platform/infra`: node dir `/data/pg-shield`, data app (PostGIS + backup CronJob), DB role, sealed secrets (`horus`), api/worker values, network policy, ingress + TLS
- [ ] S3.9 Fastlane (§7) + workflows; internal TestFlight (iOS + macOS) against the live API
- [ ] S3.10 Sentry + PostHog

### M4 — Ship to strangers
- [ ] S4.1 Store metadata, privacy label (no account, no tracking), screenshots
- [ ] S4.2 External TestFlight → App Store (universal purchase)
- [ ] S4.3 Strangers-count tracked in demand-hour. **Gate: 10 strangers before Phase 2+ polish.**

### Later phases
- **Phase 2 — Web:** SvelteKit + MapLibre on the same API (hosting decided then; recommendation in §1).
- **Phase 3 — Personal monitoring:** follow countries/regions, saved filters, APNs push (reuse `norviq-backend-v2/internal/platform/push/`), "what changed in 24h", Sign in with Apple (norviq `internal/auth` pattern).
- **Phase 4 — Android:** Kotlin + Compose + MapLibre Native.
- **Phase 5 — Layers:** licensed rain radar/precipitation, smoke, storm tracks, timeline replay, `ST_AsMVT` vector tiles, Copernicus EFFIS, cross-source merging.


## 9. Verification

- **Backend:** `make test` covers adapter fixture tests, severity table tests, sqlc/store tests against PostGIS (`TEST_DATABASE_URL`), the SSE hub test and the routes ⊆ spec test. `make lint` + `sqlc diff` clean.
- **Local E2E:** `docker run postgis/postgis:18-3.6`, `shield migrate`, `shield seed-countries`, then `shield worker` with source URLs pointed at fixture stub servers. `curl '/v1/events?country=PT&type=wildfire'` returns the expected GeoJSON with countries assigned. `curl -N /v1/stream` receives an event after a fixture changes.
- **Cluster:** all 3 Argo apps Healthy. `kubectl -n horus logs deploy/shield-worker` shows each source polled. `/v1/sources` has `last_success_at` within one interval. Series appear in Grafana Cloud. A backup CronJob dry run uploads to the Storage Box.
- **Apple:** `fastlane test` green. Simulator + macOS run against the prod API: clusters render, filters narrow results, country summary matches the API, detail links out. `fastlane beta` reaches TestFlight for both platforms.
- **Licensing:** the attribution screen lists every active source and its terms before the external TestFlight.

## Local-first development (decided 2026-11-16)

No public API host until the TestFlight build is ready to test (or the day before MVP). Until then:
- Backend runs locally: `make db-up && make migrate && make run` / `make worker`.
- The Apple app's Debug config points at `http://localhost:8080` (simulator, macOS). A physical device uses the Mac's LAN IP via a `Secrets.xcconfig` override. Debug needs an ATS exception for local HTTP; Release does not.
- The hostname is chosen at S3.9a, and Beta/Release xcconfigs get it then.


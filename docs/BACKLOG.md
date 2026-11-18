# Shield — MVP Backlog

## 8. MVP backlog

### M0 — Foundations
- [x] S0.1 Step 0 housekeeping (CLAUDE.md, memory)
- [x] S0.2 `git init` ✓, GitHub `FACorreiaa/terra-shield` ✓, README ✓, decisions doc ✓, `.gitignore` + `backend/.dockerignore` ✓, `docs/sources-licensing.md` ✓ (terms verified per source) pending
- [x] S0.3 Backend skeleton: Echo v5, config, slog, shutdown, `/health/live` `/health/ready` `/metrics` `/openapi.yaml`, `routes` subcommand, routes ⊆ spec test, `/v1/sources`
- [x] S0.4 Goose + River migrations, schema 00001, `migrate` subcommand
- [x] S0.5 sqlc setup + `testutil` template-DB harness
- [x] S0.6 `seed-countries` + auto-seed in `migrate` (Natural Earth 1:50m v5.1.2, SHA-256 pinned, ISO 3166 overrides; 237 countries, all valid)
- [x] S0.7 Dockerfile, Makefile, `ci-backend.yml` (PostGIS service, `sqlc diff`, `-race`, golangci-lint v2). `image.yml` moved to S3.9a.
- S0.8 Infra → **moved to S3.9a**. Development is local until then (see "Local-first" below).

### M1 — Ingestion
- [x] S1.1 `Source` interface + River periodic harness (unique per source) + `internal/fetch` (adapted from feeds: conditional GET, same-validator-on-200 = not modified, body cap, per-host gap, `Retry-After` → `river.JobSnooze`, capped at 1 h)
- [x] S1.2 USGS GeoJSON: `2.5_day` every 60 s (one feed instead of `all_hour` + reconcile: the day window carries every revision for 24 h, M2.5+ drops micro-quake noise). USGS ignores conditional GET, so `internal/fetch` compares `Last-Modified` itself. Quarry blasts dropped, `status=deleted` removed. Verified live 2026-11-16: 35 events, 21 within / 13 nearby / 1 none.
- [x] S1.3 GDACS: `xml/rss.xml` every 5 min with conditional GET (304 when unchanged). The JSON API is out: it caps pages at 100 and wildfires crowd out Orange/Red. TC/FL/DR/WF/VO as points; `iscurrent=false` → resolved; GDACS's country list kept in `attrs.countries`. **EQ skipped** (USGS owns earthquakes, ADR 5). Verified live 2026-11-16: 243 events.
- [x] S1.3b GDACS footprints for active Orange/Red events (`polygons/getgeometry`: TC 60 km/h whole-track buffer, FL `Poly_Affected`, DR/WF `Poly_area`), cached per episode; a failed fetch keeps the point. Verified live: the Orange South-America drought now covers 11 countries instead of 1.
- [x] S1.4 NASA EONET v3: `status=all&days=30` every 10 min (closings arrive; the open list alone has 7k never-closed records). Same-origin duplicates dropped per ADR 5 (GDACS-sourced events, earthquakes, JTWC storms); sea ice, water color, man-made skipped. Wildfire severity from burned area (provisional bands, `docs/severity.md`). Verified live: 30 IRWIN wildfires.
- [~] S1.5 Meteoalarm CAP warnings (EU): parser + adapter done (`internal/sources/meteoalarm`, 37 member feeds, awareness level → severity, Updates/Cancels delete what they reference, expired → resolved, one failing country does not fail the poll). **Not registered: needs area shapes.** Feed areas are EMMA_ID codes only; the adapter takes an `AreaResolver`. Owner chose the API route (2026-11-16). Findings: EDR `/locations/{country}` returns **only bounding boxes** per warning (FAQ), unusable for country assignment; the real shapes are the Metadata API `GET /metadata/v1/regions` (+ "Geocodes"), same token. Token auth is `Authorization: Bearer <token>` or `?token=`; the owner's token got 401 on every protected endpoint (prod and test) on 2026-11-16. It is most likely a **MeteoGate** key (`api.meteogate.eu`, `?apikey=`; the gateway answered `404 Route Not Found` for a placeholder path, so it is reachable): **retest 2026-11-17** by finding MeteoGate's MeteoAlarm warnings/regions routes. When it works: resolver = load regions once into an `emma_areas` table, `ST_Union` per warning. Token only via env `METEOALARM_TOKEN` (sealed secret in `horus`), never in code or committed files. Later: MQTT push (`warnings-<CC>` topics) replaces polling (ADR 16). Polls every 10 min until then (~30 MB/round, no validators).
- [x] S1.6 NASA FIRMS VIIRS NRT: public global 24 h CSVs (NOAA-20, NOAA-21, Suomi NPP; no MAP_KEY) every 15 min; HEAD + ETag skips unchanged 6–7 MB files; `COPY` → staging → insert new rows only, country assigned on insert; hourly purge > 48 h (ingest `Housekeeper`). Verified live: 155k detections first load in ~84 s, 99% with a country.
- [x] S1.7 Normalizer: `ST_Intersects` country assignment + 250 km `nearby` offshore fallback (migration 00002), change-only upsert (unchanged re-poll = 0 writes), revisions (S2.3), and change notification: every real change takes the next `event_change_seq` cursor + `change_op`, deletions leave tombstones, writers hold an advisory xact lock so cursor order = commit order, `pg_notify(shield_events)` fires at commit (migration 00005, ADR 20).
- [~] S1.8 `internal/severity` + `docs/severity.md` — USGS ✓, GDACS ✓, EONET wildfires ✓, Meteoalarm ✓ (FIRMS has none: detections)
- [ ] S1.9 Source health in DB + metrics + staleness alert + freshness metrics `shield_source_lag_seconds`, `shield_event_publish_seconds` (ADR 16)
- [x] S1.10 Fixtures + stubs for every adapter: USGS, GDACS (+ geometry), EONET, FIRMS, Meteoalarm

### M2 — API
- [x] S2.1 `openapi.yaml` v1 (1.0.0-draft): events list (filters, bbox, zoom clusters, GeoJSON negotiation, ETag/304), event detail (geometry, sourceInfo, revisions), countries + summary (type×severity counts, coverage, recent changes), fires (grid cells < zoom 6, points ≥ 6), SSE stream (`event.created|updated|resolved`, `Last-Event-ID`, `reset`). `severityProvisional` top-level. `docs/attrs.md` documents attrs per source. Guards: routes ⊆ spec, every `$ref` resolves, Event fields pinned to shield-core; Redocly lint valid.
- [x] S2.2 `/v1/events`: filters (type, severity, kind, status, source, country incl. nearby, since/until, limit ≤ 2000), bbox incl. antimeridian (planar, `events_geom_box_idx` migration 00003), pixel-grid clustering below zoom 6, GeoJSON via `Accept: application/geo+json` with half-pixel simplification, ETag/304 (generatedAt excluded), `max-age=15`, `Vary: Accept`, 400 naming the bad parameter. Verified live on 267 events: world view z2 = 12 events + 23 clusters, 4.4 KB gzipped, 13 ms.
- [x] S2.3 `/v1/events/{id}`: event + exact geometry + `sourceInfo` (same shape as `/v1/sources`) + newest 50 revisions; ETag/304, 400 bad UUID, 404 unknown. Verified live: Orange drought footprint (MultiPolygon, 11 countries) 27.5 KB gzipped, 21 ms.
- [x] S2.4 `/v1/countries` (237 countries: activeEvents incl. offshore nearby, maxSeverity, bbox) + `/v1/countries/{iso2}/summary` (type×severity counts, coverage, ≤100 active events, 24 h changes; one read-only snapshot). Coverage (ADR 7): `SourceInfo.Coverage`/`OfficialWarnings` → `source_coverage` + `sources.official_warnings` (migration 00004, with the `country_stats` view); officialWarnings is false everywhere until Meteoalarm is live. Verified live: list 9.2 KB gzipped; view 33 ms plan + 45 ms exec (bbox over full borders); if S2.7 flags it, store bbox columns at seed time.
- [x] S2.5 `/v1/fires` (grid below zoom 6 or without zoom: cells of 360/2^(zoom+5)° with count/maxFrp/latestAt; points from zoom 6, bbox required, ≤25k newest first + `truncated`; `since` default 24 h, clamped to 48 h; max-age 60) and `/v1/sources` (done in S2.1). Shared bbox/zoom/time parsing in `features/params`. Verified live on 103k detections: world grid 2.4 KB gzipped, zoom 3 29 KB, zoom 5 world 99 KB; Iberia points 46 in 16 ms; grid query 50 ms (acq_at index).
- [x] S2.6 `/v1/stream` SSE (ADR 20): `internal/realtime` hub per pod (one LISTEN connection, one cursor read per wake + 30 s safety poll, bounded per-client queues, slow clients dropped to reconnect), filters as `/v1/events` (bbox on the event box), `Last-Event-ID` / `?lastEventId=` catch-up of up to 1,000 coalesced changes else `reset`, `ready` cursor, `event.removed`, id-only heartbeat every 20 s, 2,000 clients/pod (503 + Retry-After), gzip skipped, closed on shutdown. Verified live: worker poll → 7 messages on an open stream; resume replayed 3; shutdown with open streams 0.16 s.
- [x] S2.7 Rate limit (20 req/s per IP, burst 40, JSON 429 + `Retry-After`; client IP trusted via Traefik `externalTrafficPolicy: Local`), 10 s request timeout except the stream (503), 8 streams per IP, CORS for `ALLOWED_ORIGINS` (ETag/Retry-After exposed, If-None-Match/Last-Event-ID allowed, no credentials), q-value Accept negotiation (JSON wins ties: generated clients list both types), gzip (already on, stream exempt). Budget `make budget` (p95 ≤ 200 ms, ≤ 300 KB gzipped, 16 map-facing requests): passes with worst p95 30 ms (country summary) and largest 38.6 KB (world GeoJSON), measured at load average ~495, so pessimistic.

### M3 — Apple app

**iOS and macOS from the first commit** (owner, 2026-11-16): one multiplatform target, and every M3 item is done only when it builds and runs on both. S3.8 is the macOS-specific layout polish, not the start of macOS support.

- [~] S3.1 Xcode project (Aaru pattern) ✓, `shield-core` (domain types, sample provider, 16 tests) ✓, tokens pipeline (`scripts/generate-tokens.mjs`, `ci-apple.yml`) ✓, core components on sample data ✓ — `shield-api` + `generate-api-client.sh` after S2.1
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
- **Phase 2 — Web:** React + TypeScript (Vite+) + MapLibre GL JS 6 in `shield-app-web/`, on the same API (ADR 19). Hosting still open: static behind Caddy, or embedded in the Go binary.
- **Phase 3 — Personal monitoring:** follow countries/regions, saved filters, APNs push (reuse `norviq-backend-v2/internal/platform/push/`), "what changed in 24h", optional sign-in with **Apple + Google + Email** (norviq `internal/auth` pattern; map, countries and detail keep working signed out per App Store 5.1.1(v); Apple ships with Google per 4.8; `DELETE /v1/me` for account deletion). Owner confirmed 2026-11-16: no accounts in the MVP.
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


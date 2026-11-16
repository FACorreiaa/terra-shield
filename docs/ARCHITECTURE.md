# Shield — Architecture

See also [DECISIONS.md](DECISIONS.md) and [BACKLOG.md](BACKLOG.md).

## Context

New product (2026-11-16): a global natural-hazard monitor answering *"What's happening on Earth right now, how serious, and where?"* Wildfires, floods/heavy rain, storms, earthquakes, volcanoes, and weather warnings, filterable by country, type, severity, kind, time and map viewport.

- Empty folder `~/Work/production/apps/shield-app/` exists. No code yet.
- User decisions (2026-11-16): **build it and add it to active projects**. MVP is the **live global map**: no accounts, no push.
- Platforms: **iOS + macOS (SwiftUI) first → web → Android later (Phase 4)**.
- Backend: **Go** on the existing Hetzner K3s cluster `maat`, deployed via `platform/infra` (ArgoCD auto-sync). No new cluster, VM, or bucket.
- Differentiator: **trusted event data**, not weather visuals. Every event shows its source, freshness, and whether it is an official warning, an observation, or a satellite detection.


## 1. Architecture

```
 Feeds (GDACS, USGS, EONET, FIRMS, Meteoalarm)
        │  conditional GET, backoff, per-host limit
        ▼
 shield worker (River periodic jobs) ── normalize → country-assign (PostGIS) → severity → upsert → revisions
        │                                                        │
        │ pg NOTIFY shield_events                                ▼
        └──────────────► shield api (Echo v5) ◄──── Postgres 18 + PostGIS (StatefulSet, horus ns)
                           REST · GeoJSON · SSE
                                 ▼
                 iOS / macOS (SwiftUI+MapKit)   ·   Web (phase 2)   ·   Android (phase 4)
```

- **One Go binary, subcommands** (norviq-backend-v2 pattern, `norviq/norviq-backend-v2/cmd/api/main.go`): `shield serve | worker | migrate [status] | routes | seed-countries`.
- **Two Helm releases from the same image** (Khepri pattern): `shield-api` and `shield-worker` (replicas 1, `strategy: Recreate`).
- **Live updates:** the worker commits and runs `NOTIFY shield_events, '<json>'`. Each API pod holds a `LISTEN` connection and fans out to SSE clients. This works across replicas and processes. Norviq's `internal/platform/realtime/bus.go` is single-replica only, so it is not reused as-is; only its handler shape is (`internal/features/groups/realtime.go`).

### Backend stack (fixed by user)
| Concern | Choice | House reference |
|---|---|---|
| HTTP | **Echo v5**: routing, middleware (request-ID, recover, CORS, rate-limit, gzip), binding + validator; auth middleware slot unused in MVP | `norviq-backend-v2/internal/server/server.go` (Root/V1 groups), `internal/middleware`, `internal/httpx`, `internal/apijson` |
| Stable queries | **sqlc** (pgx/v5) in `db/queries/<domain>/` → `<domain>db` pkgs | `norviq-backend-v2/scripts/gen-sqlc-config.sh`, CI `sqlc diff` |
| Bulk / dynamic | **pgxpool direct**: `CopyFrom` into staging tables (FIRMS, GDACS batches) then `INSERT … ON CONFLICT`; dynamic filter SQL via a small whitelisted builder | pgxpool startup with 30s ping retry (norviq) |
| Migrations | **Goose v3** `goose.NewProvider`, embedded `db/migrations/0000N_*.sql` | `norviq-backend-v2/db/embed.go`, `internal/migrate/migrate.go`; Argo PreSync `migrate --yes` |
| Jobs | **River** (`riverpgxv5`): `PeriodicJob` per source, `UniqueOpts` by source, retries/backoff, `rivermigrate` in the same migrate step | New to the house. It replaces norviq's ticker + `joblock` advisory-lock pattern. |
| Ingestion HTTP | Conditional GET (ETag / Last-Modified), exponential backoff + jitter, `Retry-After`, one in-flight request per host, timeouts, body cap (raised for FIRMS CSV) | Copy/adapt `feeds/internal/poll/{config,hostlimit}.go`, `feeds/internal/fetch/client.go` |
| Config / logs | `caarlos0/env/v11`, `slog` JSON, `signal.NotifyContext` + `Shutdown` + `RegisterOnShutdown` to close SSE | norviq `internal/config/config.go`, `cmd/api/main.go` |
| API contract | Hand-written `api/openapi.yaml` (served at `/openapi.yaml`), the source for the Swift and TS clients. `routes` subcommand + test asserts routes ⊆ spec. | norviq `internal/features/ops/feature.go` |
| Utils | `github.com/FACorreiaa/go-utils/pkg/util` (backoff, ptr helpers) where useful | `go-shared-pkg` |

### Clients
| Platform | Stack |
|---|---|
| iOS + macOS | **Aaru pattern**: hand-kept `.xcodeproj`, **one target** (`SUPPORTED_PLATFORMS = "iphoneos iphonesimulator macosx"`, `SDKROOT = auto`), `Config/{Base,Debug,Beta,Release}.xcconfig` + `Secrets.example.xcconfig`. iOS 18 / macOS 15, Swift 6, `@Observable` MV (no TCA). **MapKit**: `MKMapView` + `MKClusterAnnotation` in a representable for clustering, since SwiftUI `Map` has none (refs: `lumina/.../Places/PlacesMapView.swift`, `skyvisor/.../FlightMapView.swift`). |
| Apple API client | `shield-api/` SPM package, swift-openapi-generator (types+client, public, idiomatic), **generated code committed**. Copy `aaru/aaru-api/{Package.swift,openapi-generator-config.yaml}`, `ConnectionRetryMiddleware.swift`, `aaru/scripts/generate-api-client.sh`. Domain package `shield-core/` (like `aaru-core`). |
| Design tokens | `design/tokens.json` → `Tokens.gen.swift` via `scripts/generate-tokens.mjs` + CI check (copy Aaru `ci-brand.yml`). |
| Crash / analytics | Sentry (`sentry-cocoa`, like `aaru-ios/Aaru/CrashReporting.swift`) + PostHog (`posthog-ios`), anonymous. |
| Web (Phase 2) | SvelteKit 5 + TS + Tailwind v4 + **MapLibre GL JS** (ref `skyvisor/skyvisor-insight-client/app/assets/map.js`), `openapi-typescript` + `openapi-fetch` + `@tanstack/svelte-query` (like `north-web-app` `api:types`). **Hosting recommendation:** `adapter-static` embedded in the Go binary (Postitheka pattern, `postitheka/web/embed.go` + multi-stage Dockerfile), which keeps it on Hetzner. The alternative is the house Cloudflare Workers deploy (aaru `web-deploy.yml`). Decide at Phase 2. |
| Android (Phase 4) | Kotlin + Jetpack Compose + MapLibre Native Android, Kotlin client from the same OpenAPI spec. |


## 3. Data model (migration 00001)

- `CREATE EXTENSION postgis` (in the initdb ConfigMap run as superuser, Loci pattern), plus River tables via `rivermigrate`.
- `sources(id, name, url, license, attribution, poll_interval, etag, last_modified, last_success_at, last_error, last_error_at)`
- `countries(iso2 pk, iso3, name, geom geography(MultiPolygon,4326))` with a GIST index
- `events(id uuid, source_id, source_event_id, type, kind, status, severity, source_severity, title, summary, geom geography, centroid geography(Point), started_at, observed_at, updated_at, ended_at, fetched_at, url, attrs jsonb)`: UNIQUE(source_id, source_event_id), GIST(geom), BTREE(status, type, severity, updated_at)
- `event_countries(event_id, iso2)`, `event_revisions(event_id, at, changes jsonb)`
- `fire_detections(id, satellite, acq_at, point geography(Point), frp, confidence, iso2)`: GIST + BTREE(acq_at)
- `source_coverage(source_id, iso2)`
- `type` enum: `earthquake, wildfire, flood, storm, tropical_cyclone, volcano, drought, heat, cold, heavy_rain, wind, snow_ice, tsunami, other`


## 4. API v1 (public, read-only)

| Endpoint | Purpose |
|---|---|
| `GET /v1/events?bbox=&zoom=&type=&severity=&kind=&status=&country=&since=&until=` | JSON list or `application/geo+json`; clustered at low zoom |
| `GET /v1/events/{id}` | Detail + revisions + countries + source |
| `GET /v1/countries` / `GET /v1/countries/{iso2}/summary` | Active counts; type × severity matrix, coverage notes, recent changes |
| `GET /v1/fires?bbox=&since=` | FIRMS layer (GeoJSON) |
| `GET /v1/sources` | Health, freshness, license, attribution |
| `GET /v1/stream?bbox=&type=` | SSE `event.created \| updated \| resolved`, heartbeat, `Last-Event-ID` |
| `/health/live`, `/health/ready`, `/metrics`, `/openapi.yaml` | Ops (paths match the `norviq-app` chart's default probes) |

Event lists return `ETag` and `Cache-Control: max-age` ≤ 15 s (ADR 16: below the shortest poll interval); `/v1/sources` uses 30 s. Rate limiting is per IP.


## 5. Repo layout (monorepo, like aaru / postitheka)

```
shield-app/
├── backend/
│   ├── cmd/shield/main.go          # serve | worker | migrate | routes | seed-countries
│   ├── internal/
│   │   ├── config/ server/ middleware/ httpx/ apijson/ testutil/
│   │   ├── features/{events,countries,fires,sources,stream,ops}/
│   │   ├── ingest/                 # River workers, normalize, upsert, revisions, NOTIFY
│   │   ├── sources/{gdacs,usgs,eonet,firms,meteoalarm}/   # Fetch → []Normalized, + testdata/
│   │   ├── severity/  geo/  fetch/ # fetch = adapted from feeds (cond. GET, backoff, hostlimit)
│   ├── db/{migrations,queries,embed.go}, sqlc.yaml
│   ├── api/openapi.yaml
│   ├── Dockerfile (golang:1.27-alpine → distroless/static:nonroot), Makefile (norviq targets)
├── shield-api/  shield-core/       # Swift packages
├── shield-apple/{Shield.xcodeproj, Config/, Shield/, fastlane/, Gemfile}
├── design/tokens.json   scripts/
├── web/ (phase 2)   android/ (phase 4 placeholder)
├── .github/workflows/{ci-backend.yml, image.yml, ci-apple.yml, release-apple.yml, seed-signing.yml}
└── docs/{adr/, severity.md, sources-licensing.md}
```


## 6. Infra (`platform/infra` only; never a local `*-infra/` copy) — deferred to S3.9a, local-first until then

Template: **Khepri** (web + worker from one image, namespace `horus`). Simplest reference: Postitheka.

- `argocd/apps/shield-production.yaml`: three Applications.
  - `shield-data` → `apps/shield/data` (`prune: false`).
  - `shield-api-production` and `shield-worker-production` → chart `charts/norviq-app`, valueFiles `../../apps/shield/{api,worker}/values-{common,production}.yaml`.
- `apps/shield/data/postgres.yaml`: StatefulSet `postgis/postgis:18-3.6`, static local PV `/data/pg-shield` on `lumina-green`, initdb ConfigMap with `CREATE EXTENSION postgis`. Resources: 100m / 384Mi request, 1Gi limit.
  - **Manual one-off on the node:** `mkdir -p /data/pg-shield && chown 999:999 && chmod 700`.
  - App role per `docs/runbook-db-roles.md` + `apps/data/sql/grant-app-role.sql`.
- `apps/shield/data/backup-cronjob.yaml`: copy Loci's (pg_dump → age → rclone → Hetzner Storage Box, existing `backup-config` secret).
- API values: `migrate` PreSync enabled (runs `shield migrate --yes`), `waitForDatabase`, probes, `securityHeaders`, `prometheus.io/*` annotations, ingress host `<shield-api-host>` + `letsencrypt-prod`, TLS secret `<host>-tls`. Cloudflare DNS A record → `178.105.244.185`, grey cloud. Optional `gethomepage.dev/*` annotations.
- Worker values: `command: [shield, worker]`, replicas 1, `Recreate`, 100m / 256Mi, 768Mi limit.
- `cluster/network-policies/horus.yaml` (**matchLabels only**): Traefik → `shield-api`; observability → api + worker; `shield-api`, `shield-worker` and backup component → `shield-postgres:5432`. Egress is already open.
- `secrets/shield/`: `DATABASE_URL`, `FIRMS_MAP_KEY`, `SENTRY_DSN`, sealed for **namespace `horus`** with `printf` (bound to name + namespace). Add `*.example.yaml` + README.
- Observability: scrape annotations; `observability/alerts/shield.yaml` (source stale > 3× interval, ingest error rate, API 5xx); blackbox target in `cluster/alloy/values.yaml`; dashboard JSON later.
- CI: `image.yml` copied from `apps/postitheka/.github/workflows/image.yml` (ghcr `facorreiaa/shield:<sha>`, `sed` the tag in the infra values, open a promote PR). The infra repo's `validate.yml` requires every values file to be referenced by an Application.


## 7. Apple release pipeline (fastlane, Khepri pattern)

Copy `apps/north/north-ios-app/khepri/fastlane/{Fastfile,Matchfile,Appfile,metadata/}`:
- Constants: `RELEASE_SCHEME "Shield"`, `BETA_SCHEME "Shield TestFlight Dev"`, `BUNDLE_ID com.fernandocorreia.shield`, `BETA_BUNDLE_ID com.fernandocorreia.shield.beta`, team `84X9WYBF36`.
- Lanes: `test`, `ui_test`, `beta`, `beta_external`, `seed_signing`, `release` (phased), `hotfix`, `upload_build`, `metadata`.
- Helpers: `asc_api_key` (base64 P8), `prepare_signing`, `bump_build_number` (max of processed / TestFlight / local), `review_information` from `REVIEW_*`, `upload_dsyms` (Sentry, skipped when unset). There are no extensions in the MVP, so `prepare_signing` only maps the app target.
- **Signing constraint:** the team is at Apple's limit of 3 distribution certificates. Shield gets **its own match repo** and runs `match import` on an existing cert under its own `MATCH_PASSWORD` (as Khepri did). **Do not mint a new cert.**
- **macOS:** add a `platform :mac` block (`match(platform: "macos")`, `build_app(... destination macOS)`, `upload_to_app_store(platform: "osx")`) to ship as a universal purchase. Crib from `aaru/aaru-ios/fastlane/Fastfile`, the only house Fastfile that already ships macOS from a single target.
- Workflows on `macos-15`, copied from aaru: `ci-apple.yml`, `release-apple.yml`, `seed-signing.yml`. Secret names as in Khepri.



# Shield

What's happening on Earth right now, how serious is it, and where?

Shield is a global natural-hazard map: earthquakes, wildfires, floods, storms, volcanoes and official weather warnings, normalized from GDACS, USGS, NASA EONET, NASA FIRMS and Meteoalarm. Every record shows its source, the source's own severity next to Shield's derived one, and how fresh it is. Shield does not replace national emergency services.

- **Plan:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/DECISIONS.md](docs/DECISIONS.md), [docs/BACKLOG.md](docs/BACKLOG.md)
- **Platforms:** web (React + TypeScript on Vite+, launches first, ADR 22), iOS + macOS (SwiftUI) in parallel, then Android.
- **Deploys** from `platform/infra` (LuminaVaultInfra) to the `maat` cluster, namespace `horus`. Merging this repo deploys nothing.

## Layout

| Path | What |
|---|---|
| `backend/` | Go: `shield serve` (Echo v5 API), `shield worker` (River ingestion), `shield migrate` (goose + River) |
| `backend/api/openapi.yaml` | API contract; Swift and TS clients are generated from it |
| `docs/` | Architecture, decisions, backlog |
| `shield-apple/`, `shield-api/`, `shield-core/` | Apple app and Swift packages (M3) |
| `shield-app-web/` | Web app: React + TypeScript on Vite+, MapLibre (ADR 19, 22) |
| `android/` | Phase 4 |

## Backend quick start

```sh
cd backend
make db-up        # PostGIS 18 on :55440 (amd64 image, emulated on Apple Silicon)
make migrate
make run          # API on :8080
make worker       # ingestion worker, /metrics on :9090
make test         # uses TEST_DATABASE_URL against the same container
```

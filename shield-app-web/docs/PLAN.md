# Aegis Terra Web — Implementation Plan

> **For agentic workers:** use superpowers:executing-plans (or subagent-driven-development) slice by slice. Steps use `- [ ]`.

**Goal:** Build a complete, production-grade Aegis Terra web dashboard in `shield-app-web/`. It mirrors the iOS/macOS app's design and behaviour, consumes the Go API, and runs locally.

**Architecture:** React 19 + TanStack Router/Query on Vite+ (ADR 19). MapLibre GL JS 6 draws an OpenFreeMap vector basemap restyled from tokens, with globe/flat projection. Pure domain logic is ported from `shield-core` with matching tests. One SSE stream per browser is shared across tabs. Saved regions and the Alert center are browser-local.

**Tech stack:** Vite+ (`vp`), bun, TypeScript 7, React 19, TanStack Router + Query, Tailwind v4 (generated tokens), shadcn/ui primitives, MapLibre GL 6, terra-draw (region drawing), Vitest + happy-dom, Playwright + @axe-core/playwright.

**Spec:** `DESIGN.md`, `VIEWS.md`, `docs/DECISIONS.md` (ADR 2–9, 16, 18–20, 22, 27, 28), `docs/attrs.md`, `docs/severity.md`, `backend/api/openapi.yaml` (origin/main). The Apple code is the reference behaviour: `shield-core/Sources/ShieldCore/*`, `shield-apple/Shield/{Map,Components}/*`.

## Context

The owner made the web app the first thing to launch (ADR 22, 2026-11-19), so it gates the API go-live (S3.9a). `shield-app-web/` holds only a spike: a Natural Earth map, a flat event list, `SeverityBadge`, `KindChip`, freshness/place helpers and a generated API client. The Apple app has done S3.1–S3.7: map, filters, country, detail, live SSE, sources, onboarding, empty states. The web must reach parity with that, plus:

- landing hero at `/`
- Saved regions and Alert center, browser-local
- OpenFreeMap basemap with a globe/flat toggle
- live flights and conflict layers (merged in #35 and #40)

### Owner decisions (2026-11-19, this session)

1. **Basemap:** OpenFreeMap vector tiles (commercial use OK, no key, no limits; terms checked 2026-11-19). The custom style is self-hosted, bundled in the app. Attribution: "OpenFreeMap © OpenMapTiles Data from OpenStreetMap". Natural Earth stays as the offline/error fallback.
2. **Projection:** user option for globe or flat, persisted. Defaulting to globe is my choice, not the owner's.
3. **Saved regions + Alert center:** build them now, ahead of DESIGN's Phase 3 gate. They are browser-local because no API exists. Peers informed.
4. **Entry:** landing page at `/`, live map at `/map`.

### Coordination (sent and answered)

- **Builder session** (`build-shield-hazard-monitor`) owns `backend/` and docs/{ARCHITECTURE,BACKLOG,DECISIONS,severity,attrs,alerts,sources-licensing}.md. It regenerates `schema.gen.ts` whenever `openapi.yaml` changes, so we never hand-edit it. Nothing of theirs is in flight. Their constraints for us:
  - Share one stream across tabs (cap is 8 streams per IP).
  - Develop flights against `go run ./cmd/aircraft-replay` with `AIRCRAFT_FEED_URL=http://127.0.0.1:18091`, never the live quota (500/day).
  - Gate aircraft on view size (≤ 4 cells of 5°), not zoom; poll ≥ 60 s.
  - Conflicts: `observedAt` is the end date; only `attrs.locationPrecision == "exact"` is a real point.
  - Local API on :8080 runs main; the Vite `/v1` proxy works as is.
- **Design session** (`aegis-terra-ui-design-docs`) owns `scripts/generate-tokens.mjs` and `design/tokens.json`. Web token output already exists. Add tokens there and regenerate; never edit `.gen` files.
  - Copy the Apple S3.7 deviations: Sources dot rule (amber = last check failed and newer than last success, red = server `stale`), disabled sources hidden, "Checked every N" / "Monthly release · checked every 6 h", no `termsUrl` → link `url`.
  - Text on `accent` uses `canvas`, never white.
  - **They asked me to make the DESIGN.md §9/§10.4/§10.5/§17 and VIEWS.md surface-map edits in my branch.**
- **To send at slice 1:** amendment text for the builder to record in DECISIONS (ADR 19: OpenFreeMap tiles; Phase 3 web-local override) and the README's stale "web (SvelteKit)" line. That file is theirs.

## Global constraints

- **Copy is verbatim from DESIGN §2.**
  - Kind words: **Observed**, **Official warning**, **Forecast**, **Satellite heat detection**.
  - Disclaimer: _"Aegis Terra does not replace your national emergency service. Always follow official instructions."_ It has one source: `DisclaimerBanner`.
  - Never use exclamation marks, emoji, "No threats", or "fire" for a FIRMS detection.
- **Two severities, never merged.** The badge reads `▲ Severe · GDACS Orange`. With no source value it reads "Source: no level".
- **Severity encoding is always colour + shape + word.** Kind changes form, never colour.
- **No hex in components.** Colours come only from `tokens.gen.css` / `tokens.gen.ts`, including the MapLibre style, which is built from tokens at runtime.
- **Non-live numbers** carry `SampleDataBadge`.
- **Freshness** appears everywhere, as "12 min ago · 14:02 UTC".
- **Dark only.** Respect `prefers-reduced-motion` (cross-fades, static double ring) and `prefers-contrast: more` (hairlineStrong, opaque panels).
- **Hit targets** ≥ 44 px. Focus ring is `accent`.
- **Conflicts:** opt-in everywhere. Copy: "Conflict data are monthly research estimates (UCDP), not live reports".
- **Unknown `type`** → `other`. Ignore unknown keys.
- **Tests never call third-party hosts.** Playwright stubs `tiles.openfreemap.org/**` and the API.
- **No deploy, Cloudflare, secrets or hostnames** in this plan.
- **Repo rules:**
  - Work in worktree `.worktrees/web`, branch `web/<slice>` off `origin/main`.
  - Only touch `shield-app-web/` and DESIGN.md/VIEWS.md. Docs ship in the slice whose code they describe.
  - **Exception: tokens.** The web CSS has no material, elevation, alpha or `hairlineStrong` tokens; checked 2026-11-19, the generated file ends at motion. W1 adds them to `design/tokens.json` plus the web branch of `scripts/generate-tokens.mjs`, then regenerates every output, Swift included. The design session must OK this before I start.
  - PR titles follow the house style `web: … (W<n>)`.
  - Approving this plan authorises one commit + PR per slice.

## Review focus (failure modes no happy-path test hits)

1. **Antimeridian and globe bounds.** `getBounds()` under globe at low zoom, and bboxes crossing ±180°, must still produce a valid server bbox (whole world when span ≥ 180°). Test in `viewport.test.ts`.
2. **Stream gap and reset.** A tab sleeps, the cursor is > 1,000 behind → `reset` → reload, with no duplicate or phantom alerts. Bursts > 50 changes/s fold into one reload. Test in `liveStore.test.ts`.
3. **Multiple tabs.** Three tabs open one EventSource, not three. When the leader closes, another tab takes over with the same cursor. e2e `multitab.spec.ts`.
4. **Tiles unreachable.** The basemap falls back to Natural Earth and the hazard layers still render. e2e with the tile host aborted.
5. **Corrupt or old localStorage** (filters, regions, alerts) loads defaults, never a crash. Zod-parsed, versioned keys. Unit tests per store.

---

## File structure (target, `shield-app-web/apps/web/src/`)

```
domain/        pure, ported from ShieldCore, 100% unit-tested, no React
  taxonomy.ts      types/categories/severity rank+labels, kind words, OpenEventType→other
  sort.ts          sortedForDisplay (active>resolved, severity desc, updatedAt desc)
  freshness.ts     (exists) + stamp (date beyond a day), resolvedWords, isStale
  place.ts         (exists) phrase / nearbyWords
  eventFacts.ts    severityRating, severityNote, showsProvisional, keyFigures, detailTimes, locationNote, arrivalLine, Figures.number
  filters.ts       EventQuery, defaults, activeFilters, removing, label, toggling, toApiQuery, windowSince
  countries.ts     matching, normalizedCode, mapBox (antimeridian), highSeverityCount, warningCount, matrix, Change.parse/sentence
  sources.ts       listed, health, statusLine, interval
  statStrip.ts     liveMap cells with lower bounds ("12+", "–")
  viewport.ts      viewOf(map) → {zoom, bbox}, world clamp, aircraft gate (≤4 cells of 5°)
  regions.ts       Region model, contains(region, event) (iso2 / bbox / point-in-polygon on centroid)
  alerts.ts        deriveAlerts(change, regions, prev) → Alert[]
lib/
  api/client.ts    openapi-fetch (exists) + typed error → LoadFailure ("You're offline." etc.)
  storage.ts       versioned, zod-parsed localStorage helper (try/catch, defaults on failure)
  live/            streamLeader.ts (Web Locks + BroadcastChannel), sse.ts, liveStore.ts
features/
  map/         HazardMap, basemapStyle (tokens→style), fallbackStyle (NE), layers/{events,areas,clusters,fires,aircraft,selection,halo}.ts, markerImages, MapControls, LayersMenu
  monitor/     StatStrip, StatusCapsule, FilterChips, FilterSheet, EventRow, EventList, MapSearch, NewEventToast, MapNotices
  detail/      EventDetailView, KeyFigures, RevisionTimeline, EventMapSnippet
  country/     CountryView, TypeSeverityMatrix, CoverageNotice, CountriesList
  sources/     SourcesView, SourceRow, HealthDot
  regions/     SavedRegionsView, SavedRegionCard, RegionThumbnail (SVG), RegionEditor, RegionDetail
  alerts/      AlertCenter, AlertRow, AlertPreferences, alertStore
  landing/     LandingPage
  settings/    SettingsPage
components/    SeverityBadge, KindChip, SourceBadge, FreshnessLabel, DisclaimerBanner, EmptyState, SampleDataBadge, BottomSheet, Inspector, ui/*
app/shell/     DesktopShell (sidebar · map · inspector), MobileShell (map · capsule · sheet), useBreakpoint, shortcuts.ts
routes/        index (landing), _app (layout: persistent map), _app.map, _app.e.$id, _app.countries.index, _app.countries.$iso2 (+ c.$iso2 redirect), _app.sources, _app.saved, _app.saved.$regionId, _app.alerts, _app.settings
```

Brand art is copied from `design/brand/` into `apps/web/public/brand/`:

- Lockup, scaled.
- Sentinel B.
- The four backgrounds as SVG.

---

## Slices (one PR each; each ends with `vp run ready` + e2e green + screenshots at 1280 px and 390 px compared against DESIGN §10 states)

### W1 — Rails

- [ ] Create the worktree. Run `vp install`.
- [ ] Remove the unused deps: three, @react-three/_, @deck.gl/_, react-hook-form, @hookform/resolvers. A grep of `src/` confirmed there are no imports.
- [ ] Add terra-draw (+ the maplibre adapter), @axe-core/playwright and zod (already present; now used).
- [ ] Verify MapLibre 6's projection API (`setProjection({type:"globe"|"mercator"})`, sky/atmosphere) in `node_modules` before W2 relies on it.
- [ ] Routes per the file structure. `_app` is a pathless layout that keeps one `HazardMap` mounted across `/map`, `/e/$id`, `/countries/$iso2`, … . Route-level code splitting; MapLibre loads only under `_app`.
- [ ] `lib/storage.ts` + tests (corrupt JSON, wrong version → defaults).
- [ ] Domain ports with tests, each mirroring the matching ShieldCore test cases: `taxonomy`, `sort`, `freshness` (+stamp, resolvedWords, isStale), `eventFacts`, `filters`, `countries`, `sources`, `statStrip`, `viewport`.
- [ ] Move the existing e2e from `/` to `/map`.
- [ ] **Token gap.** Add `material.floating`, `elevation.*`, `alpha.*` (matrixTint, area fills) and `hairlineStrong` to the CSS/TS outputs via tokens.json and the generator (see the Repo rules exception).
- [ ] **terra-draw check.** Confirm the MapLibre adapter's peer range covers maplibre-gl 6. If it doesn't, hand-roll rectangle and polygon drawing in W9 instead.
- [ ] **Docs.** VIEWS.md surface map gets the web routes. Build-order item 9 becomes React, not Svelte.
- [ ] Copy `PLAN.md` into `shield-app-web/docs/`.
- [ ] Send the ADR amendment text to the builder session.

### W2 — Map core

- [ ] **Basemap.** `basemapStyle(tokens)` builds the OpenMapTiles-schema style:
  - Source `https://tiles.openfreemap.org/planet`, glyphs `…/fonts/{fontstack}/{range}.pbf`, no sprite.
  - Water a step darker than land; admin-0 `hairline`; labels `faint`.
  - No roads below z8, no POIs.
  - Atmosphere edge on globe.
  - Attribution bottom-right.
- [ ] **Fallback.** On a source error, swap to the Natural Earth style (existing `basemap.ts`).
- [ ] **Projection.** Setting persisted (`shield.map` v1). Toggle lives in `LayersMenu`.
- [ ] **Layers,** bottom to top (DESIGN §9): areas (by kind: observed 18%/1.5 px, warning 10%/2 px, forecast dashed [6,4]) → fires → clusters → points → selection ring → halo.
  - Points: a symbol layer with images per severity × kind × state (stale/resolved: 45% opacity, desaturated, clock/check badge), Lucide glyph inside the shape, scale per severity.
  - Clusters: DOM buttons (SVG: panel disc, max-severity ring, proportional arc, mono count, aria-label "Cluster, 14 events, highest severity extreme"). Click fits the cluster bbox.
  - Extreme halo: DOM ring for extreme events in view, `extremePulse`; static double ring under reduced motion.
- [ ] **Viewport.** `viewport.viewOf` handles globe bounds and the antimeridian. Debounce moveend at 250 ms.
- [ ] **Tests:** unit tests for `basemapStyle` (no hex outside tokens, roads minzoom ≥ 8) and `viewOf`; e2e for the tile-host-aborted fallback.
- [ ] **Docs:** DESIGN §9 records OpenFreeMap and the projection toggle (owner, 2026-11-19).

### W3 — Shells

- [ ] **`useBreakpoint`:** the split is at 1024 px.
- [ ] **DesktopShell** (≥ 1024, after Mac.dc.html):
  - Sidebar 240–260 px: nav mark + `AEGIS TERRA` micro, nav (Monitor, Countries, Saved regions, Alerts with an unread badge), the section body (Monitor: search + list; Countries: searchable list), Sources pinned at the bottom with the overall health dot.
  - Map canvas with `StatStrip` top-left, `FilterChips` top-centre and `MapControls` top-right (layers, locate, fit world, zoom ±), all on `floating` material.
  - Right `Inspector`, 320–380 px, floating material, toggle `i`.
- [ ] **MobileShell** (< 1024, after Main.dc.html): `StatusCapsule` + chips on top, plus a `BottomSheet`.
  - Detents: peek ≈ 96 px (search + compact strip), medium ≈ 50%, large ≈ 92%.
  - Drag with velocity projection; spring 0.35 s / 0.86.
  - Map interactive at peek and medium.
  - Keyboard: the handle is a button that cycles detents.
  - Focusing search opens large.
  - Header buttons: Sources, Saved, Alerts, Settings.
  - Spike this first: it is the riskiest UI piece. Use the apple-design skill.
- [ ] **Shortcuts** (`shortcuts.ts`): web-safe single keys, active only outside inputs. Recorded as a deviation, because ⌘1–5, ⌘L, ⌘R and ⌘F are browser-reserved.

  | Key       | Action                             |
  | --------- | ---------------------------------- |
  | `1`–`5`   | Navigation                         |
  | `/`       | Search                             |
  | `i`       | Inspector                          |
  | `o`       | Open official source               |
  | `r`       | Refresh                            |
  | `0`       | Fit world                          |
  | `h`       | Heat layer                         |
  | `↑` / `↓` | List navigation                    |
  | `Esc`     | Clear search → selection → country |
  | `?`       | Help sheet                         |

- [ ] **Tests:** e2e at 1280 and 390 (`BottomSheet` detents; Escape order); `BottomSheet` unit test for detent snapping under velocity.
- [ ] **Docs:** VIEWS web deviations list gets the shortcuts mapping.

### W4 — Monitor: list, strip, filters, states

- [ ] **Data hooks.** `useEvents(view, query)` (exists; extend with filters), `useSources`, `useCountries`.
- [ ] **Shared components,** to VIEWS props:
  - `EventRow`, `EventList`: severity then recency; footnote "N events inside clusters — zoom in".
  - `StatStrip`: full and compact. Tapping High severity or Official warnings toggles that filter.
  - `StatusCapsule`, `FreshnessLabel`, `SourceBadge`, `SampleDataBadge`, `EmptyState` (mascot or radar), `DisclaimerBanner` (inline/footer), and the web footer disclaimer.
- [ ] **Filters.** `FilterSheet` (popover on desktop, sheet over sheet on mobile) + `FilterChips`:
  - Type grid grouped by `category`, conflicts off by default.
  - Minimum severity as a segmented control with shapes.
  - Kinds: four toggles; Satellite heat = the fires layer.
  - Window: 1 h, 6 h, 24 h, 7 d.
  - Status: "Include resolved".
  - "Reset" when anything differs from the default.
  - Stored as exclusions under `shield.filters` v1, same semantics as `FilterStore`.
- [ ] **`MapSearch`.** Countries (`countries.matching`) + events in view by title. **Places:** none, because no geocoder exists (open owner item).
- [ ] **§10.1 states:**
  - Loading: progress line + skeleton rows on Radar Grid.
  - Empty: "No active events in this view from covered sources" + coverage + "Zoom out". With filters on: "No events in this view match your filters" + Reset.
  - Stale: capsule in `warning`.
  - Offline: "Offline · showing data from 14:02" + Retry; nothing cleared.
  - Error: inline `danger` banner + Retry; the map never blanks.
- [ ] **Tests:** e2e per state with route stubs; axe check with no violations.

### W5 — Detail, Country, Sources

- [x] **`EventDetailView`** (inspector / sheet large; deep link `/e/$id`), per §10.3 and the accepted S3.5 deviations:
  - Header: glyph in shape, title, every country as a link; type art at 30% under a vignette (Precipitation / Wildfire SVG).
  - `SEVERITY · AEGIS / SOURCE` row; rating labels and provisional note.
  - Kind, status, times.
  - `KeyFigures` (attrs only; tsunami bands; conflict precision + UCDP copy).
  - `EventMapSnippet` (small non-interactive MapLibre) + Show on map.
  - Source block: "Checked" + Open official source (`o`).
  - `RevisionTimeline`: rename-only revisions dropped; "Area updated".
  - Footer: disclaimer + Share (copies the `/e/{id}` URL).
  - States: stale banner, resolved (`ok` chip, desaturated header), gone (404 → "This event is no longer published by its source" + source home).
- [x] **`CountryView`** (`/countries/$iso2`, `/c/$iso2` redirects; plus the Countries sidebar list with active counts):
  - Header: name, ISO mono, freshness. Map fits `mapBox` (antimeridian rule).
  - `StatStrip`: Active · Severe+ · Official warnings ("100+" when capped).
  - `TypeSeverityMatrix`: cells filter the list.
  - `CoverageNotice` + "Absence of data is not safety." once.
  - Recent changes (24 h, sentences).
  - Events: "Near Chile, 38 km offshore".
  - Mascot empty state; not-found "We have no boundary for this place."
- [x] **`SourcesView`** (`/sources`): disclaimer on top; `SourceRow` + `HealthDot` per the design session's rules; refresh on open; keeps the last list if a refresh fails.
- [ ] **Tests:** unit tests on the facts and timeline words; e2e for detail deep link / gone / resolved, the country matrix filter, and the sources dot rule.

### W6 — Live

- [x] **`lib/live/feedNode.ts`** (was `streamLeader.ts`). `navigator.locks.request("shield-stream")` elects one tab. That tab reads `/v1/stream` with `fetch` (not `EventSource`: it hides the id-only heartbeat and `Retry-After`; see VIEWS W6 deviations), sending `Last-Event-ID`:
  - Unfiltered (hazards only), like the Apple `LiveStream`. Conflicts are never streamed: they are monthly data (W7).
  - Native auto-reconnect is safe. The server prefers the `Last-Event-ID` header over the query (`stream.go:102`), so a stale `?lastEventId=` on reconnect is ignored.
  - Relays messages over `BroadcastChannel("shield-live")`.
  - Persists the cursor (`shield.live` v1).
  - Followers take over when the leader exits. A follower that hears no relayed message or heartbeat for 45 s pings the leader. With no answer, it requests `{steal: true}` (frozen or background-throttled leader).
  - Fallback when Locks/BroadcastChannel are missing: a per-tab stream.
- [x] **`domain/livePatch.ts` + `app/shell/useLiveChanges.ts`** (was `liveStore.ts`).
  - created/updated → upsert into the TanStack Query `events` caches when the event matches filters and view.
  - removed → drop.
  - `reset` → invalidate.
  - More than 50 changes/s → one reload.
  - Reload every 60 s regardless.
  - "Live" ends after 45 s of silence.
- [x] **Arrival.** Fade + scale from 0.6 over 400 ms; fade only under reduced motion; live arrivals only, not the reconnect backlog. (A DOM copy of the marker animates while the symbol waits, then hands over.)
- [x] **`NewEventToast`.** Severe+ events, in view and matching filters, get one line from `arrivalLine`. Lasts 6 s; tap opens the event; `aria-live="polite"`.
- [x] **Stale.** Driven by `/v1/sources` (refetched every 60 s).
- [x] **Tests:** unit tests on liveStore (upsert, burst fold, reset, out-of-view ignored); e2e with a stubbed SSE route (arrival + toast; multi-tab single stream).

### W7 — Layers: heat detections, flights, conflicts

- [ ] **Fires.** `/v1/fires`: grid → heatmap below z6 (`detection` tint, weight = count); points ≥ z6 as 3–6 px dots. Copy: "Detected 3 h ago · VIIRS". Inspector shows satellite, confidence, FRP. Off by default.
- [ ] **Aircraft.** `/v1/aircraft?bbox=`:
  - Only when `viewport.aircraftAllowed`; else a "Zoom in to see aircraft" notice; a 400 gets the same notice.
  - Poll every 60 s while visible.
  - Heading-rotated glyph in `muted`/`ink` (not a hazard colour); military marked by shape.
  - `dataAt` age shown, `complete:false` noted.
  - Body `attribution` added to the map attribution.
  - Sheet/inspector: callsign, altitude, speed.
  - Dev only against aircraft-replay.
- [ ] **Conflicts.** Their own query, `useConflicts(view)`: `category=conflicts`, no `since`, server-clustered like events. The time window and status filters don't apply, because conflicts are active and dated by month (`observedAt` = end date; checked live 2026-11-19: 2,000+ returned only without `since`). Not streamed; refetched every 6 h. Never alerted. The category menu opts in. Non-exact precision is drawn as a hollow approximate ring labelled "Approximate location". Copy is fixed (see Global constraints). Never in hazard counts.
- [ ] **Tests:** unit tests for the aircraft gate; e2e for layer toggles with stubs.

### W8 — Landing (`/`)

- [ ] **Global header and footer (owner, 2026-11-25).** A thin header on every page, the map included: brand, nav (Map, Countries, Sources, Alerts), Settings. A full footer (disclaimer, Sources, Settings, attributions) on the landing and static pages; the map keeps its disclaimer strip. Settings itself stays in W9.

- [ ] **Hero.** Lockup on Orbital Earth SVG under a radial vignette, plus `GLOBAL HAZARD INTELLIGENCE` and "Global hazard intelligence. Official sources, live." CTA "Open the live map" → `/map`.
- [ ] **Live `StatStrip`.** From `/v1/events?zoom=1` (events + cluster counts summed) and a second `&kind=warning` query for Official warnings (checked: cluster counts sum to real numbers). Lower-bound rules apply; never sample numbers.
- [ ] **"Know what you're looking at"** section: the four `KindChip`s + Sentinel. This replaces onboarding card 2.
- [ ] **Sources strip** (names + attribution) and the disclaimer. This replaces card 3.
- [ ] **Footer:** disclaimer, Sources, Settings.
- [ ] **No MapLibre on this route.** Budget: LCP < 2.5 s on local preview, initial JS ≤ 200 KB gz.
- [ ] **Meta tags:** title, description, OG image (lockup), `lang="en"`, favicon set from the shield crop.
- [ ] **Tests:** e2e (CTA, disclaimer present, no network to the tile host).

### W9 — Saved regions + Alert center (browser-local) + Settings + docs

- [ ] **`regions.ts` / `regionStore`** (`shield.regions` v1).
  - Region: `{id, name, area: {country: iso2} | {bbox} | {polygon}, filters: {types, minSeverity, kinds}, prefs: {notify, quietHours?, extremeBypass}}`. Hazards only.
  - Default filters: official warnings + observed, ≥ moderate, detections off.
  - `contains`: country by `event.countries` iso2; drawn by centroid in bbox/polygon (events carry no bbox).
- [ ] **`RegionEditor`.**
  - Draw a rectangle or polygon with terra-draw on the main map, or pick a country.
  - Name it and set filters.
  - Max 25 regions.
- [ ] **`SavedRegionsView`** (`/saved`).
  - `SavedRegionCard`: SVG thumbnail from Natural Earth 110m (2:1, `radius.md`), "Active: 4 · Severe+: 1", top-right `SeverityBadge`, last change.
  - Sorted by highest severity, then name.
  - Empty: mascot, "Watch the places that matter to you", Add region.
- [ ] **`RegionDetail`** (`/saved/$regionId`).
  - Country region → reuses `CountryView`.
  - Drawn region → `/v1/events?bbox=<bounds>&zoom=6&limit=2000`, then the centroid filter, with strip, matrix and list.
  - **No "recent changes"** section, because no API feeds it. Recorded as a deviation.
- [ ] **`alerts.ts` / `alertStore`** (`shield.alerts` v1, cap 500).
  - The leader tab derives alerts from stream changes: a new event, or one raised in severity or into kind warning, that matches a region's filters → `Alert{id, eventId, regionId, severity, kind, urgency?, certainty?, at, read, replayed}`.
  - **Baseline** (`shield.alertBaseline` v1): per region, event id → the highest severity already alerted or seen. The stream sends only current state, so "raised" means above the baseline. A replayed change at or below the baseline alerts nothing.
  - The baseline is seeded silently from a `/v1/events` load of the region when it is created, and again after a `reset`.
  - Urgency/certainty come only from `attrs` (MeteoAlarm); otherwise the row reads "Not given by source" in `muted`. Never invented.
  - Broadcast to all tabs.
- [ ] **`AlertCenter`** (`/alerts`).
  - Rows grouped Today / Yesterday / Earlier; unread accent dot; click or swipe marks read.
  - "Mark all read" skips extreme.
  - Row opens the detail.
  - Header copy: "Alerts are checked while Aegis Terra is open in a browser tab."
  - After a resume, replayed alerts are labelled "While you were away". On `reset`, one notice: "Some changes may have been missed while no tab was open."
  - Empty copy per §10.5.
- [ ] **`AlertPreferences`** (in region detail).
  - Minimum severity, kinds, quiet hours, extreme bypass (opt-in).
  - Browser notifications: permission is asked only when the user enables it. Defaults are conservative: no alerts below severe.
- [ ] **`SettingsPage`** (`/settings`).
  - Projection, layer defaults, notifications status.
  - "Clear local data": filters, regions, alerts.
  - About: lockup, version, disclaimer, attributions (OpenFreeMap/OSM, Natural Earth, each source).
- [ ] **Docs:**
  - DESIGN §10.4/§10.5/§17: web-local override, owner 2026-11-19.
  - VIEWS deviations: no place search, drawn regions without recent changes, alerts only while a tab is open.
- [ ] **Tests:**
  - Unit: `contains` (antimeridian bbox, polygon), `deriveAlerts` (raised above baseline alerts; equal or below doesn't; replay after resume makes no duplicates; reset reseeds silently; quiet hours with extreme bypass).
  - e2e: create region → stubbed SSE event → alert row + badge.

---

## Verification

- **Per slice:** `cd shield-app-web && vp check && vp test && vp run -r build && vp run e2e`. All green; axe reports 0 serious/critical violations.
- **Live, local:**
  1. Builder's API on :8080 (`/health/ready` ok). For flights, start aircraft-replay and set `AIRCRAFT_FEED_URL`.
  2. Run `vp run dev`.
  3. Drive Chrome (claude-in-chrome) at 1280 × 800 and 390 × 844. Screenshot every DESIGN §10 screen and state.
  4. Check numbers against `curl`/API (country summary counts match).
  5. Leave a tab open while the worker polls and see an SSE change arrive.
  6. Open three tabs and confirm exactly one `/v1/stream` request in the network log.
- **Owner verdicts:** screenshots plus a deviation list per slice in the PR body. Agents draft pixels; the owner keeps verdicts.
- **Skills at build time:** `impeccable` (dashboard polish/critique), `apple-design` (sheet, springs, materials), `dataviz` (stat strip, matrix, cluster arc).

## Open items for the owner (not blocking)

- Place search needs a geocoder: Nominatim's usage policy forbids heavy use; Photon or a paid service would be needed. Until then, countries + events only.
- Alerts only fire while a tab is open. Real background alerts need Web Push + an API (Phase 3).
- Recording the OpenFreeMap / Phase-3 amendments in DECISIONS (builder's file): the text will be sent to the builder.

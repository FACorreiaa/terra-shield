# Aegis Terra — Views

This file lists the surfaces and the component catalog. `DESIGN.md` says what Aegis Terra should feel like; this file says
what gets built, what each component takes, which API route backs it, and in what order.

`docs/DECISIONS.md` bounds everything here: two severities never merged, `kind` on every
record, coverage shown, freshness visible, attribution in the app, and a permanent disclaimer. API
routes are those in `docs/ARCHITECTURE.md` §4. If a route changes there, update this file.

---

## Surface map

| Surface | iOS | macOS | Web (Phase 2) | Route(s) | Phase |
|---|---|---|---|---|---|
| Monitor (live map) | Root: full-screen map + sheet | Sidebar "Monitor" → map canvas | `/` | `GET /v1/events`, `GET /v1/stream`, `GET /v1/fires` | MVP |
| Event detail | Sheet (large detent) | Inspector | Right panel / sheet; deep link `/e/{id}` | `GET /v1/events/{id}` | MVP |
| Country overview | Sheet (large) | Sidebar "Countries" list → content + inspector | `/c/{iso2}` | `GET /v1/countries`, `GET /v1/countries/{iso2}/summary` | MVP |
| Filters | Sheet over sheet | Toolbar popover | Popover / sheet | (local state) | MVP |
| Sources & attribution | From capsule menu | Sidebar "Sources" | `/sources` | `GET /v1/sources` | MVP |
| Onboarding | First run, 3 cards | First run window | Landing hero | none | MVP |
| Settings | From capsule | ⌘, window | `/settings` | (local) | MVP |
| Saved regions | Sheet header bookmark | Sidebar "Saved regions" | `/saved` | Phase 3 API (TBD) | **Phase 3, gated** |
| Alert center | Sheet header bell | Sidebar "Alerts" | `/alerts` | Phase 3 API (TBD) | **Phase 3, gated** |

The iPhone has no tab bar: the map keeps the screen and everything else is a sheet route. The macOS sidebar order
is Monitor · Countries · Saved regions · Alerts, with Sources pinned at the bottom (⌘1–⌘5).

---

## Component catalog

Each component is built once in SwiftUI (in the app target's `Design/` folder, using `shield-core` models)
and later once in Svelte with the same name and props. Props are named after API fields.

| Component | Props | Job | Source |
|---|---|---|---|
| `HazardMarker` | `type, severity, kind, isStale, isSelected` | Point on map: severity shape + colour, type glyph inside, kind form (§5) | events |
| `ClusterMarker` | `count, maxSeverity, severityShares` | Panel circle, ring in max severity, proportional arc, mono count | events (server clusters at low zoom) |
| `EventArea` | `geometry, severity, kind` | Polygon/line with fill + stroke by kind | events GeoJSON |
| `DetectionLayer` | `points, visible` | FIRMS, own layer, off by default. Heatmap at low zoom, dots at regional zoom (~155k points per 48 h). "Detected N h ago" | `GET /v1/fires` |
| `SeverityBadge` | `severity, sourceSeverity?, isProvisional, size` | Shape + word + colour; "Provisional" when derived from a weak signal; source value verbatim in muted | events |
| `KindChip` | `kind` | Fixed word + icon; dashed border for forecast | events |
| `SourceBadge` | `source{name, attribution, license}, url?` | Who said so | events.source, sources |
| `FreshnessLabel` | `observedAt, updatedAt, fetchedAt, pollInterval` | "12 min ago · 14:02 UTC"; stale → warning + clock | events, sources |
| `StatusCapsule` | `isLive, lastUpdate, isOffline` | iOS top capsule: nav mark, live dot, freshness | stream state |
| `StatStrip` | `cells[{value?, isLowerBound, label, unit?, filter?}], style: full\|compact, query?, tap?, isSample` | Numbers-first summary; "12+" or "–" where clusters cannot say; compact is one line for the iPhone peek; in the full strip, tapping High severity or Official warnings toggles that filter | derived from current list / country summary |
| `MapSearchField` + `MapSearchResults` | `search: MapSearch`, `model` | Search places (MapKit completer, moves the map) and events in view (title); iPhone peek and macOS sidebar (⌘F) | MapKit; events in view |
| `FilterChips` | `query, openFilters, remove` | A Filters button (count when on), then one removable chip per filter that differs from the default; iPhone under the capsule, Mac top-centre | local |
| `FilterSheet` | `model` | Type grid, min severity (shapes), kinds + the heat layer, time window; "Reset" when not default. Status waits for multi-value `status` on `/v1/events` (backend, after S1.5) | local (`FilterStore`) |
| `EventRow` | `event, source?, isStale, isSelected` | Glyph-in-shape, title, kind chip, source, freshness | events, sources |
| `EventList` | `model` | The viewport's `EventRow`s, severity then recency, with loading, empty and failed states; one view for the iOS sheet and the sidebar, because the list is the accessible alternative to the map (DESIGN §14) | events |
| `EventSheet` / `EventInspector` | `eventId` | Event detail layout (DESIGN §10.3); one view, two hosts | `GET /v1/events/{id}` |
| `KeyFigures` | `type, attrs` | Type-specific figures; render only those present | events.attrs |
| `RevisionTimeline` | `revisions[]` | Changes in words + time, newest first | events/{id}.revisions |
| `CountrySummaryCard` | `summary` | Country header + stat strip | countries summary |
| `TypeSeverityMatrix` | `matrix, onSelect` | Type × severity counts | countries summary |
| `CoverageNotice` | `coverage[], uncovered[]` | "Absence of data is not safety" | countries summary, sources |
| `RecentChanges` | `changes[]` | Last 24 h of revisions in a place | countries summary |
| `SourceRow` | `source` | Name, coverage, licence, attribution, health dot | `GET /v1/sources` |
| `DisclaimerBanner` | `style: inline \| footer` | Fixed disclaimer copy (DESIGN §2) | none |
| `NewEventToast` | `event` | One-line top toast for severe+ arrivals in view | stream |
| `MapControls` | `layers, onLocate, onFitWorld` | Floating material buttons | local |
| `EmptyState` | `title, message, action?, art: mascot \| radar` | Sentinel mascot or radar grid, coverage-aware copy | none |
| `SampleDataBadge` | none | Required on every non-live number | none |
| `SavedRegionCard` | `region, summary` | Mini-map, counts, `SeverityBadge`, last change | Phase 3 |
| `RegionEditor` | `geometry?, filters` | Draw / pick country, name, filters | Phase 3 |
| `AlertRow` | `alert{severity, urgency, certainty, kind, region, source, at, read}` | CAP axes as separate labelled fields | Phase 3 |
| `AlertPreferences` | `region, prefs` | Min severity, kinds, quiet hours | Phase 3 |

Rules:
1. **No hex in a component.** Colours, radii, spacing and motion come from generated tokens only.
2. **Every event-showing component shows kind + source + freshness**, or sits inside one that does.
3. **Severity components always take both severities.** If `sourceSeverity` is missing, show "Source: no level".
4. **One view, two hosts.** `EventSheet` and `EventInspector` share the same content view. The same
   goes for Country.
5. **Previews use fixtures marked sample.** `#Preview` data comes from `shield-core` fixtures,
   and any preview screenshot shows `SampleDataBadge`.
6. **A new component** is added to this table with its reason before it ships. Before that, prefer
   the nearest existing one.

---

## Build order

Client work follows the API: endpoint, then DTO in `shield-api`, then view. This matches BACKLOG M3.

1. **S3.1 Rails.** Generate tokens from `design/tokens.json` (copy `apps/aaru/scripts/generate-tokens.mjs` +
   `ci-brand.yml`). Add the app icon from the logo crop (DESIGN §11) and `SeverityBadge`, `KindChip`,
   `SourceBadge`, `FreshnessLabel`, `SampleDataBadge` with previews.
   *Built 2026-11-16, uncommitted:*
   - `shield-apple/Shield.xcodeproj` is one iOS + macOS target, with Config/*.xcconfig.
   - `shield-core` has the domain types, `HazardEventProvider` and `SampleEventProvider`, with tests.
   - `scripts/generate-tokens.mjs` writes colour sets and `Tokens.gen.swift`, and `--check` runs in CI.
   - `.github/workflows/ci-apple.yml` runs the token check, a colour-literal guard, the core tests and the
     iOS/macOS builds.
   - The app's root is `ComponentGallery` on sample data. `HazardMarker` was pulled forward from S3.2.
   - `shield-api` is generated from `backend/api/openapi.yaml` (v1.0.0-draft) by `scripts/generate-api-client.sh`,
     and CI fails when it is stale. `APIEventProvider` maps the generated types onto `shield-core`. The app keeps
     `SampleEventProvider` until the backend serves `/v1/events` (S2.2); the swap is one line in `ShieldApp`,
     plus linking `shield-api` in the Xcode project.
2. **S3.2 Map.** `HazardMarker`, `ClusterMarker`, `EventArea`, `DetectionLayer`, `MapControls`,
   `StatusCapsule`, `StatStrip`, `EventRow`, and the iOS sheet with three detents.
   *Status 2026-11-18:* map, markers, server + local clusters, `EventArea` (selected event, by
   kind, stale desaturated), `DetectionLayer` (density below zoom 6, dots from 6), `MapControls`,
   `StatusCapsule` and every §10.1 state are built and checked against the local API.
   `StatStrip`, `EventRow` and the shells are built too, checked on sample data in the iPhone
   simulator only (the macOS and iPad shell compiles but has not been looked at):
   - `StatStrip` shows observed events, high severity and official warnings. Counts include
     cluster members. Clusters carry no kind and sometimes no severity split, so those cells
     read "12+" or "–" instead of guessing. Cells are display-only until S3.3 gives them a filter.
   - `EventList` (`EventRow`s, most severe first, then newest) is the map's accessible
     alternative. Clustered events are not listed; a footnote says how many.
   - iPhone (compact width): a sheet with three detents. Peek is `MapSearchField` over the
     compact `StatStrip` (≈ 96 pt, measured, so it follows Dynamic Type), medium is the full
     strip and the list or the selected event, large is the same with room for S3.5's detail.
     Focusing search opens large. Selecting on the map
     or in the list opens the event at medium. The map's attribution moves to sit above the
     sheet at peek and at medium; at large the sheet covers the map.
   - macOS and regular width (iPad): `NavigationSplitView` (sidebar 220–260 pt) with search
     (⌘F), `StatStrip` and list in the sidebar, and the selected event as a card on the map.
     The map repeats the empty and truncated notices only when the sidebar is hidden.
   - Search (`MapSearch`) has no endpoint behind it: places come from MapKit's completer
     (address results only) and move the map; events match by title among those in view, so
     clustered events are not searched (a hint says so). Countries arrive with S3.4.
   - The sample-data badge is the map's own, so it stays in view in every sheet state; the
     strip no longer repeats it. Extreme markers pulse on the map only, not in rows. The full shell (sidebar navigation,
     inspector, shortcuts) is still S3.8.

   DEBUG launch env for screenshots: `SHIELD_DETECTIONS=1`, `SHIELD_REGION=lat,lon,latSpan,lonSpan`,
   `SHIELD_SELECT=<event id>`, `SHIELD_SHEET=peek|medium|large`.
3. **S3.3 Filters.** `FilterChips`, `FilterSheet`, persisted. **Built** (2026-11-19), except status:
   - `EventQuery` filters live in shield-core with the chip words (`activeFilters`, `label(for:)`,
     `removing(_:)`) and the strip's quick filters (`toggling(_:)`), all tested.
   - `FilterStore` saves to `UserDefaults` what is *excluded*, so a type or kind added later shows
     by default; unreadable data gives the defaults. Heat detections are saved with them.
   - The model refuses a query with no type or no kind (`/v1/events` reads an empty list as all).
   - An empty view with filters on says so ("No events in this view match your filters") and
     offers Reset filters, since the filters, not the sources, may be what hides events (§15).
   - iPhone presents the filter sheet from inside the map sheet; the Mac as a window sheet.
   - DEBUG: `SHIELD_RESET_FILTERS=1`, `SHIELD_FILTER_DEMO=1`, `SHIELD_SHOW_FILTERS=1`.
   - Open for the owner: "Observed events" still counts every kind once a kind filter is on.
4. **S3.4 Country.** `CountrySummaryCard`, `TypeSeverityMatrix`, `CoverageNotice`, `RecentChanges`.
5. **S3.5 Detail.** `EventSheet`/`EventInspector`, `KeyFigures`, `RevisionTimeline`, and the
   type backgrounds (DESIGN §12).
6. **S3.6 Live.** `NewEventToast`, the arrive animation, and stale handling driven by SSE + sources.
7. **S3.7 Sources.** `SourceRow`, `DisclaimerBanner`, onboarding, and `EmptyState` with the mascot.
8. **S3.8 macOS.** `NavigationSplitView` shell, inspector, keyboard shortcuts (DESIGN §10).
9. **Phase 2 Web.** The same catalog in Svelte, plus the MapLibre style (DESIGN §9).
10. **Phase 3 (gate: 10 strangers).** `SavedRegionCard`, `RegionEditor`, `AlertRow`,
    `AlertPreferences`, after the accounts + push API exist.

---

## Open

- **Navy basemap.** MapKit can't be recoloured (DESIGN §9). The options are to keep MapKit muted dark
  (the current call), move Apple to MapLibre Native with the same style as the web, or add a
  licensed dark tile overlay. This needs an owner verdict after seeing S3.2 on device.
- **App icon crop.** Owner sign-off on the shield crop on `canvas` (DESIGN §11).
- **Trademark / licence** for "Aegis Terra" and the AI-generated art, before M4 store submission.
- **Pointers for the backend session** (not edited from the design session):
  ARCHITECTURE "Clients → Design tokens" should point to `design/tokens.json` + `DESIGN.md`, and
  BACKLOG S3.1 should name the token generator copy and the app-icon crop.

# Aegis Terra — Views

This file lists the surfaces and the component catalog. `DESIGN.md` says what Aegis Terra should feel like; this file says
what gets built, what each component takes, which API route backs it, and in what order.

`docs/DECISIONS.md` bounds everything here: two severities never merged, `kind` on every
record, coverage shown, freshness visible, attribution in the app, and a permanent disclaimer. API
routes are those in `docs/ARCHITECTURE.md` §4. If a route changes there, update this file.

---

## Surface map

| Surface | iOS | macOS | Web (launches first, ADR 22) | Route(s) | Phase |
|---|---|---|---|---|---|
| Monitor (live map) | Root: full-screen map + sheet | Sidebar "Monitor" → map canvas | `/map` | `GET /v1/events`, `GET /v1/stream`, `GET /v1/fires` | MVP |
| Event detail | Sheet (large detent) | Inspector | Right panel / sheet; deep link `/e/{id}` | `GET /v1/events/{id}` | MVP |
| Country overview | Sheet (large) | Sidebar "Countries" list → content + inspector | `/c/{iso2}` | `GET /v1/countries`, `GET /v1/countries/{iso2}/summary` | MVP |
| Filters | Sheet over sheet | Toolbar popover | Popover / sheet | (local state) | MVP |
| Sources & attribution | From capsule menu | Sidebar "Sources" | `/sources` | `GET /v1/sources` | MVP |
| Onboarding | First run, 3 cards | First run window | Landing hero at `/` (owner, 2026-11-19), replaces the cards | none | MVP |
| Settings | From capsule | ⌘, window | `/settings` | (local) | MVP |
| Saved regions | Sheet header bookmark | Sidebar "Saved regions" | `/saved`, browser-local (owner, 2026-11-19) | Phase 3 API (TBD); web: none | **Phase 3, gated**; web built now |
| Alert center | Sheet header bell | Sidebar "Alerts" | `/alerts`, browser-local (owner, 2026-11-19) | Phase 3 API (TBD); web: `/v1/stream` | **Phase 3, gated**; web built now |

The iPhone has no tab bar: the map keeps the screen and everything else is a sheet route. The macOS sidebar order
is Monitor · Countries · Saved regions · Alerts, with Sources pinned at the bottom (⌘1–⌘5).

---

## Component catalog

Each component is built once in SwiftUI (in the app target's `Design/` folder, using `shield-core` models)
and once in React (`shield-app-web/`, ADR 19) with the same name and props. Props are named after API fields.

| Component | Props | Job | Source |
|---|---|---|---|
| `HazardMarker` | `type, severity, kind, isStale, isSelected` | Point on map: severity shape + colour, type glyph inside, kind form (§5) | events |
| `ClusterMarker` | `count, maxSeverity, severityShares` | Panel circle, ring in max severity, proportional arc, mono count | events (server clusters at low zoom) |
| `EventArea` | `geometry, severity, kind` | Polygon/line with fill + stroke by kind | events GeoJSON |
| `DetectionLayer` | `points, visible` | FIRMS, own layer, off by default. Heatmap at low zoom, dots at regional zoom (~155k points per 48 h). "Detected N h ago" | `GET /v1/fires` |
| `SeverityBadge` | `severity, sourceSeverity?, isProvisional, size` | Shape + word + colour; "Provisional" when derived from a weak signal; source value verbatim in muted | events |
| `KindChip` | `kind` | Fixed word + icon; dashed border for forecast | events |
| `SourceBadge` | `source{name, attribution, license}, url?, style: compact\|full, checkedAt?` | Who said so; `full` adds attribution · licence, "Checked" (the source's last poll) and `OfficialSourceButton` (accent, ⌘L) | events.source, sources |
| `FreshnessLabel` | `observedAt, updatedAt, fetchedAt, pollInterval` | "12 min ago · 14:02 UTC"; stale → warning + clock | events, sources |
| `StatusCapsule` | `isLive, lastUpdate, isOffline` | iOS top capsule: nav mark, live dot, freshness | stream state |
| `StatStrip` | `cells[{value?, isLowerBound, label, unit?, filter?}], style: full\|compact, query?, tap?, isSample` | Numbers-first summary; "12+" or "–" where clusters cannot say; compact is one line for the iPhone peek; in the full strip, tapping High severity or Official warnings toggles that filter | derived from current list / country summary |
| `MapSearchField` + `MapSearchResults` | `search: MapSearch`, `model` | Search places (MapKit completer, moves the map) and events in view (title); iPhone peek and macOS sidebar (⌘F) | MapKit; events in view |
| `FilterChips` | `query, openFilters, remove` | A Filters button (count when on), then one removable chip per filter that differs from the default; iPhone under the capsule, Mac top-centre | local |
| `FilterSheet` | `model` | Type grid, min severity (shapes), kinds + the heat layer, time window; "Reset" when not default. Status waits for multi-value `status` on `/v1/events` (backend, after S1.5) | local (`FilterStore`) |
| `EventRow` | `event, source?, isStale, isSelected, place?` | Glyph-in-shape, title, kind chip, source, freshness; `place` says "Near Chile, 38 km offshore" where the list around it is a country (ADR 4) | events, sources |
| `EventList` | `model` | The viewport's `EventRow`s, severity then recency, with loading, empty and failed states; one view for the iOS sheet and the sidebar, because the list is the accessible alternative to the map (DESIGN §14) | events |
| `EventDetailView` (the catalog's `EventSheet` / `EventInspector`) | `event, state: LiveMapModel.DetailState?, source?, isStale, countryName, openCountry?, showOnMap?, close` | Event detail layout (DESIGN §10.3); one view, two hosts: the iPhone sheet (from medium) and the Mac/iPad `.inspector`. Loading, loaded, gone, failed; stale and resolved | `GET /v1/events/{id}` |
| `KeyFigures` | `figures` (`HazardEvent.keyFigures`) | Type-specific figures; render only those present; the lead one large | events.attrs |
| `RevisionTimeline` | `state` (`EventDetail.timeline`) | Changes in words + time, newest first; rename-only revisions left out | events/{id}.revisions |
| `CountryView` | `overview, model, close, selectEvent` | The overview (built as one view, the catalog's `CountrySummaryCard` + `RecentChanges`): name, ISO code, freshness; Active · Severe+ · Official warnings; matrix; coverage; changes in the last 24 h; the events. Loading, not-found ("We have no boundary for this place"), failed, and failed-with-data states | countries summary |
| `TypeSeverityMatrix` | `rows, selection` | Type × severity counts; columns show shape + name; a cell with events is tinted (`alpha.matrixTint`) and filters the list; one line per type where it fits, else name above cells (Mac sidebar, AX sizes) | countries summary |
| `CoverageNotice` | `countryName, coverage, sources` | Official warnings yes/no, the sources by name, "Absence of data is not safety." Names no missing service: the API doesn't say which | countries summary, sources |
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
     clustered events are not searched (a hint says so). Countries were added with S3.4.
   - The sample-data badge is the map's own, so it stays in view in every sheet state; the
     strip no longer repeats it. Extreme markers pulse on the map only, not in rows. The sidebar sections and ⌘1–⌘5
     came with S3.8 (build order 8).

   DEBUG launch env for screenshots: `SHIELD_DETECTIONS=1`, `SHIELD_REGION=lat,lon,latSpan,lonSpan`,
   `SHIELD_SELECT=<event id>`, `SHIELD_SHEET=peek|medium|large`.
3. **S3.3 Filters.** `FilterChips`, `FilterSheet`, persisted. **Built** (2026-11-19); status added
   the same day:
   - `EventQuery` filters live in shield-core with the chip words (`activeFilters`, `label(for:)`,
     `removing(_:)`) and the strip's quick filters (`toggling(_:)`), all tested.
   - `FilterStore` saves to `UserDefaults` what is *excluded*, so a type or kind added later shows
     by default; unreadable data gives the defaults. Heat detections are saved with them.
   - The model refuses a query with no type or no kind (`/v1/events` reads an empty list as all).
   - An empty view with filters on says so ("No events in this view match your filters") and
     offers Reset filters, since the filters, not the sources, may be what hides events (§15).
   - iPhone presents the filter sheet from inside the map sheet; the Mac as a window sheet.
   - Status: "Include resolved" sends `status=active,resolved`, shows a "With resolved" chip, and is
     saved (an optional key, so filters saved before it still load). Resolved events are faded with
     a `muted` checkmark on the map (no pulse, below active ones), sort last in lists, and read
     "Resolved · ended 50 min ago" in `ok` where a row or card would show freshness. The time
     window applies to their last update.
   - The strip never counts resolved events. Server clusters don't say how many of theirs are
     resolved, so while resolved events are included, "Observed events" and "High severity"
     show "–" wherever clusters are on screen (Open: a cluster status split).
   - DEBUG: `SHIELD_RESET_FILTERS=1`, `SHIELD_FILTER_DEMO=1`, `SHIELD_SHOW_FILTERS=1`,
     `SHIELD_RESOLVED=1`.
   - Open for the owner: "Observed events" still counts every kind once a kind filter is on.
4. **S3.4 Country.** `CountrySummaryCard`, `TypeSeverityMatrix`, `CoverageNotice`, `RecentChanges`.
   **Built** (2026-11-19), from search only:
   - shield-core: `Country`, `CountrySummary` (matrix, `highSeverityCount`, `warningCount`,
     `notableChanges`), `Change.parse` with the words ("Raised to Severe · area updated"),
     `Country.matching` (codes, then name prefixes, then contains; accents ignored),
     `Country.mapBox(for:)`, and `CountryOverview` (one per country opened, so a late answer
     for another can't land; failed reloads keep the last summary). All tested, plus API tests
     against a real `/v1/countries/HR/summary`.
   - Numbers: Active and Severe+ come from the counts, so they're exact. Official warnings come
     from the listed events, which the server caps at 100, so they read "100+" when capped. The
     list then says "Showing the 100 most severe of 156 events."
   - Events offshore of the country say "Near Chile, 38 km offshore", never "in" (ADR 4).
   - A bbox crossing ±180° (US, RU, FJ, NZ, KI, AQ: nearly 360° wide) frames the country's
     events instead, by the smallest arc of longitude; with none, its band of latitude.
   - Revisions that only renamed their event are hidden. S1.5 gave every MeteoAlarm title its
     area, and fifty "renamed" rows buried the one real change.
   - An event opened from the overview goes back to it on close. `LiveMapModel.select(_:)`
     shows an event even when the map's filters or a cluster leave it out.
   - The map frames the country above the sheet. Doing so exposed a MapKit trap: with the safe
     area added to the map's layout margins, a medium sheet (seen with a country open) left the
     margins taller than the map, a region with no height, and no events. Only the sheet's cover
     and the home indicator count now.
   - Mac: the overview replaces the sidebar's strip and list; Escape closes it.
   - DEBUG: `SHIELD_COUNTRY=PT`.
   - **Not built:** the long-press entry (needs a gesture that can't be checked here), the country
     name in event detail (S3.5), the Countries sidebar (S3.8), and the mascot `EmptyState`
     (S3.7). "No active events in X" carries that state for now.
   - **Deviations for the owner** (DESIGN §10.2):
     - The overview opens at medium, not large, so the country stays in view.
     - No 60% scrim outside the country: that needs its boundary, and the API gives a bbox.
     - The coverage notice names no missing service ("Not covered: national flood service"):
       nothing in the data says which one is missing.
5. **S3.5 Detail.** `EventSheet`/`EventInspector`, `KeyFigures`, `RevisionTimeline`, and the
   type backgrounds (DESIGN §12). *Built (2026-11-19), as `EventDetailView`:*
   - Order per §10.3: header (glyph in shape, title, every country as a link to its overview),
     `SEVERITY · AEGIS / SOURCE` with the source text wrapping beside the badge, the backend's
     labels ("Rated by wind speed · 241 km/h", "Rated by burned area · 48,484 ha", "Prescribed
     burn") and one line on why, kind + status + times, the source's summary, key figures,
     Show on map, the source block, the timeline, the disclaimer and Share.
   - Tsunamis (#25): a PTWC forecast says "Guidance for national authorities, not a warning.",
     that its marker is the epicentre, and leads with its highest wave band, then each band's
     places; a US warning lists its coasts and the zones the map can't draw. DEBUG:
     `SHIELD_GALLERY=tsunami`.
   - `LiveMapModel.selectedDetail` drives it, so an event stays open when it leaves the
     snapshot (panned away, joined a server cluster); a merge into a local MapKit cluster is unchecked. DEBUG: `SHIELD_SELECT=<id>`, `SHIELD_GALLERY=detail`.
   - **Not built:** the map snippet (Show on map frames the main map instead), the §12 header
     art, and a public share link (Share sends the source's record until the web app has one).
   - Map snippet (`EventMapSnippet`): the area drawn by kind on a still map, tap = Show on map.
     The type's §12 art sits behind the header at 30%, faded out to transparent.
   - **Deviations, accepted by the owner 2026-11-19** (DESIGN §10.3):
     - Mac/iPad show it in an `.inspector` (320–380 pt) rather than the old card on the map.
     - A source's retraction keeps the detail open as gone instead of closing it.
     - The timeline says "Area updated", not "Area grew": the API only flags a geometry change.
     - The source block shows "Checked" (the source's last poll), not the event's fetched time:
       `fetched_at` only moves when the record changes, so it read "Fetched 7 h ago" on a
       source polled minutes before.
     - Gone sends Open official source to the source's home page, not the event's dead page.
6. **S3.6 Live.** `NewEventToast`, the arrive animation, and stale handling driven by SSE + sources.
   *Built (2026-11-19):* the toast sits under the map's top row (capsule, chips, controls), one
   line, severity-tinted edge; the arrival fade is Core Animation on the marker view, so it works
   the same on the Mac.
7. **S3.7 Sources.** `SourceRow`, `DisclaimerBanner`, onboarding, and `EmptyState` with the mascot.
   *Built (2026-11-19): `SourcesView`, `SourceRow` (+ `HealthDot`), `DisclaimerBanner`.* Gallery:
   `SHIELD_GALLERY=sources`; screen: `SHIELD_SHOW_SOURCES=1`.
   - **Deviations for the owner:**
     - Entry: the sheet header (per DESIGN §10 shells), not the capsule menu this table names.
     - Dot rule, which §10.7 leaves open: last check failed = `warning`, stale = `danger`.
     - Disabled sources aren't listed ("every active source").
     - The interval says "Checked every 6 h" (our poll), like the detail's "Checked".
   *Part 2 (2026-11-19): `OnboardingView`, `EmptyState` (mascot only; the radar art waits for the
   loading skeleton).* Gallery: `SHIELD_GALLERY=empty`; cards: `SHIELD_ONBOARDING=1|2|3`.
   - **Deviations for the owner:**
     - Card 3 has a "Before you start" heading above the disclaimer.
     - The cards replace the map until finished (the iPhone map's sheet can't sit under a modal).
     - The empty-country line reads "No active events in Iceland from covered sources."
     - The empty state on the map itself (sheet hidden) keeps words only: no mascot on the map (§11).

**Conflicts layer, Apple (ADR 27; built 2026-11-19).** Owner verdicts pending:
- Glyph `scope` (web: Lucide `crosshair`) in `tokens.json → hazardType.conflict`.
- Conflict markers use the severity shape and colour like hazards, plus a dashed ring when the
  location is approximate. Conflict clusters get a dashed ring and a small `scope` badge.
- The toggle sits in the map controls above heat detections. No category menus (still unapproved).
- The layer ignores the hazard filters: every conflict UCDP still lists, minor and up.
- The list shows conflicts in their own section after the hazards; the stat strip stays hazards only.
- The adsb.lol row on Sources reads "Fetched only while the flights layer is shown, at most once a
  minute" and "Not fetched yet" / "Fetched 5 min ago" (`onDemand`, #43).
8. **S3.8 macOS.** `NavigationSplitView` shell, inspector, keyboard shortcuts (DESIGN §10).
   Built 2026-11-25 (macOS and iPad regular width share the shell). Owner verdicts pending:
   - The sidebar lists **Monitor** and **Countries** on top and **Sources** at the bottom with the
     sources' overall health dot (worst listed source; on-demand ones never stale; no dot before the
     first list). **Saved regions and Alerts are left out**, not shown disabled, until Phase 3;
     ⌘3 and ⌘4 stay reserved.
   - ⌘1, ⌘2, ⌘5 live in a **Go** menu (discoverable; not yet pressed with the app in front). ⌘5 and the Sources row
     open the existing Sources sheet; Sources did not become a content pane.
   - Monitor keeps search, strip and list. Countries is a filter field over every country: those
     with active events first (most severe, then busiest; severity mark + count), then the rest by
     name. Choosing one opens the existing country overview in the sidebar; choosing the section
     again goes back to the list. Changing section closes an open country and any search. ⌘F
     switches to Monitor and focuses its search.
   - Not done: ⌥⌘I (the inspector follows the selection) and the toolbar's time-window, layers
     and share menus.
   - Window: it opened 1121 pt tall, taller than a 1084 pt screen, and could not be made shorter.
     AppKit took the window's minimum from the sidebar measured at almost no width, where the
     vertically fixed texts (stat strip, list footnote) wrap a word per line. The sidebar column now
     takes the height it is given (minimum 480): the window opens at 1200 × 800 and shrinks to 592.
     The 4647 pt width no longer reproduces from a clean state (fresh bundle id, no saved frame).
   - DEBUG: `SHIELD_SECTION=2` selects a section at launch (screenshots on iPad).
9. **Web (launches first, ADR 22).** The same catalog in React + TypeScript (ADR 19), plus the MapLibre style (DESIGN §9). Plan and slices W1–W9: `shield-app-web/docs/PLAN.md`.
10. **Phase 3 (gate: 10 strangers).** `SavedRegionCard`, `RegionEditor`, `AlertRow`,
    `AlertPreferences`, after the accounts + push API exist.

---

## Open

- **Navy basemap.** MapKit can't be recoloured (DESIGN §9). The options are to keep MapKit muted dark
  (the current call), move Apple to MapLibre Native with the same style as the web, or add a
  licensed dark tile overlay. This needs an owner verdict after seeing S3.2 on device.
- **Cluster status split.** With resolved events included, `/v1/events` clusters count active and
  resolved together, so the strip shows "–" at low zoom. A per-cluster resolved count would let it
  show real numbers (asked of the backend, 2026-11-19).
- **App icon crop.** Owner sign-off on the shield crop on `canvas` (DESIGN §11).
- **Trademark / licence** for "Aegis Terra" and the AI-generated art, before M4 store submission.
- **Pointers for the backend session** (not edited from the design session):
  ARCHITECTURE "Clients → Design tokens" should point to `design/tokens.json` + `DESIGN.md`, and
  BACKLOG S3.1 should name the token generator copy and the app-icon crop.

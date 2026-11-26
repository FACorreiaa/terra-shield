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
| Country overview | Sheet (large) | Sidebar "Countries" list → content + inspector | `/countries`, `/countries/{iso2}` (`/c/{iso2}` redirects) | `GET /v1/countries`, `GET /v1/countries/{iso2}/summary` | MVP |
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
   - **W3 shells.** The split is at 1024 px. At 1024 px and wider the web uses the Mac layout: a 248 px
     sidebar, the map, and a 352 px inspector that floats over the map's right edge while something is
     selected. Below 1024 px it uses the phone layout: status capsule and a bottom sheet with detents
     at peek, 50% and 92%. The sheet is dragged by its header. A release is thrown to the detent its
     velocity projects to, on a 0.35 s / 0.86 spring. A tap or Enter on the handle cycles peek → half
     → full → peek; ↑/↓ step one detent. The map stays mounted across both layouts, and the camera
     and attribution keep clear of the chrome.
   - **Deviations for the owner:**
     - **Single-key shortcuts.** The browser owns ⌘1–5, ⌘L, ⌘R and ⌘F, so the web uses single keys,
       live only outside text fields, the map canvas and dialogs: `1`–`5` sections, `↑`/`↓` list,
       `Esc` clear selection, `o` official source, `i` inspector, `0` whole world, `r` refresh, `h`
       heat detections (W7, for ⌥⌘H), and `?` for a sheet that lists them all.
     - **Search key.** `/` focuses search (W4, part 2), standing in for ⌘F.
     - **Sidebar nav lists built sections only.** It shows only the sections that exist, so Monitor
       alone for now.
     - **No locate button.** See W4.
     - **Deferred to W4:** `StatStrip` and `FilterChips` over the map and the compact strip at peek
       (built in W4, part 1); the rule that focusing search opens the sheet at full height (W4, part 2).
     - **Deferred to W9:** the sheet header's Saved, Alerts and Settings buttons, and the sidebar's
       Countries, Saved regions and Alerts entries.
     - **Owner call:** `i` hides the inspector until `i` is pressed again, across new selections, and
       nothing on screen brings it back. Options: show it again on the next selection, or add a
       visible toggle.
     - **Tap target:** a tap anywhere on the sheet's header outside the search field, not only on
       the grabber, steps the detent, so the target is far larger than 44 px.
   - **W4 Monitor, part 1.** `StatStrip` (full, and compact at the phone's peek), the status capsule's
     states, `FilterChips` and the filter sheet, the list's states, and the error banner, after the
     apps' `StatStrip`, `FilterChips`, `FilterSheet`, `EventList` and `MapScreen`.
     - **Copy is the apps':** "Loading events…", "Offline · showing data from 14:02 UTC", "Updated
       2 min ago", and "Last update 14 min ago" after three missed 60 s refreshes. Load failures read
       "Shield’s server isn’t responding." or "Something went wrong loading events.", never the
       server's own text. The empty view reads "No active events in this view from covered sources."
       (or "…match your filters." with Reset filters), then "Absence of data is not safety. Official
       warnings come from national services.", with Zoom out. The footnote counts clustered events.
     - **Nothing is cleared.** A failed load for a new view or new filters keeps the last events on
       the map and in the list, with the banner. Offline, requests wait and the capsule says from
       when the data is.
     - **Filters persist** under `shield.filters` as the apps' own versioned blob, shared between
       tabs. A stat cell toggles its filter, marked by an accent underline.
   - **Deviations for the owner:**
     - **Flat type grid.** The type grid is flat, like the apps and DESIGN §10.6, not grouped by
       category as the web plan said. Conflicts are a layer of their own (W7), never in the grid.
     - **Desktop layout.** On a wide screen the chips sit under the strip at the map's top left,
       not top centre, so they never meet the controls or the inspector at 1024 px.
     - **Phone header.** At half and full height the compact strip hides but keeps its line, so the
       peek height never shifts; the full strip heads the sheet's body.
     - **Chip height.** Chips are 44 px tall (the apps use 32 pt) to meet the hit-target rule.
     - **Offline capsule on a phone.** Offline, the capsule keeps the mark but drops the wordmark,
       so "Offline · showing data from 14:02 UTC" and Retry fit beside it.
     - **Owner verdict:** at half and full height the hidden compact strip leaves a blank band under
       the search field in the sheet's header, the cost of a peek that never shifts.
     - **Locate, owner call.** The apps have no locate button, and one would ask for the person's
       location. It is not built; it needs a yes first.
     - **`SampleDataBadge`** has no use on the web yet: it shows no sample data.
   - **W4 Monitor, part 2: search.** One field, after the apps' `MapSearchField` and
     `MapSearchResults`: under the nav in the sidebar on a wide screen, in the sheet's header on a
     phone. It finds countries from `/v1/countries` (by name or ISO code, ignoring case and accents)
     and events in view by title, in the list's order; events grouped on the map aren't searched,
     and a hint says so. Choosing an event selects it. Choosing a country ends the search, clears
     the selection and, once the sheet or inspector has settled, fits the map to the country and its
     events in view (never narrower than half a degree, as in the apps). On a wide screen results
     wait for text, as on the Mac, so the list stays and Tab walks on into it; keyboard focus goes
     back to the field when the search ends, or to the chosen event's row. On a phone, focusing the
     field opens the sheet at full height, and Cancel or ending the search puts it back if it is
     still there (at half height when an event was chosen from collapsed); results take the sheet
     over a selection. `/` focuses the field, Escape ends the search before it clears the
     selection, and ↑/↓ do nothing while results show. The result count is spoken as it changes.
   - **Deviations for the owner:**
     - **No places.** The apps search places with MapKit's completer. The web has no geocoder, so it
       searches countries and events only, and the placeholder reads "Search countries or events".
       A geocoder (Photon, or a paid one) is an open item in the web plan.
     - **Phone peek.** The field takes the place of the visible "Live events" title, which stays as
       a heading for screen readers. Peek is the field over the compact strip.
     - **A failed country list** shows no countries and no error; events stay searchable.
     - **Device check.** The field uses a 16 px font so iOS Safari doesn't zoom on focus. The soft
       keyboard over the sheet at full height still needs a check on a real phone.
   - **W5, part 1: event detail.** `EventDetailView` after `EventDetailView.swift`, in the
     inspector on a wide screen and in the sheet on a phone. Sections in order: what and where (marker,
     title, every country, location note, the type's art at 30% under a vignette), `SEVERITY · AEGIS /
     SOURCE`, kind with status and times, the source's summary, key figures, the area on a small
     still map with "Show on map", the full source badge with "Open official source", the timeline,
     then the disclaimer and Share. The selection lives in the address (`?event=<id>` on every
     section), so Back closes it, a reload keeps it and switching section keeps it open beside the
     section, as on the Mac; `/e/<id>` redirects there. A page opened on an event frames it on the map
     once known, and on a phone opens the sheet at half height. The detail loads from
     `/v1/events/{id}` and keeps the list's copy while it loads; a failed refresh keeps the last
     detail. A 404 or 400 is gone: the event stays shown faded, with "This event is no longer
     published by its source", "Open official source" pointing to the source's home page, and no
     history. A stale source (from `/v1/sources`, every 60 s) adds the stale banner and fades the
     marker; a resolved event shows its resolution in `ok` and a muted header.
   - **Deviations for the owner:**
     - **A link to an event never seen here** that answers 404 shows only "This event is no longer
       published by its source" and Close. The apps drop such a selection; on the web links are how
       people arrive, so it says why the page is empty.
     - **Share copies the web link** (`/e/<id>`) and says "Link copied"; there is no system share
       sheet on desktop browsers.
     - **The small map's credit** sits under it as plain links: MapLibre's control inside a button
       would nest interactive elements.
     - **Close replaces the history entry**, so opening then closing leaves one extra Back step
       on the map (Back after opening closes in one step). Going back instead would pop pages the
       user came from on a shared link. Owner call.
     - **Header art** is drawn at 30% and fades out radially; over the dark panel the Wildfire art
       reads as a soft red glow behind the title. Owner verdict wanted.
   - **W5, part 2: country overview and Countries.** After `CountryView.swift` and the Mac's
     Countries sidebar. `/countries` (key 2) lists every country with a boundary, those with active
     events first (highest severity, then most events, then name) with the severity shape and the
     count, the rest under "No active events", with its own filter field; the map's search field
     steps aside there. `/countries/<iso2>` (`/c/<iso2>` redirects) shows the overview from
     `/v1/countries/{iso2}/summary`, refreshed every 60 s: name with the ISO code, "Updated …",
     Active · Severe+ · Official warnings ("100+" when capped), the type × severity matrix (a cell
     filters the events below, with a chip to remove it), the coverage notice with "Absence of data
     is not safety." once, changes in the last 24 h (four, then "Show all N"), and the events, each
     "Near Chile, 38 km offshore" where it is offshore. A capped list says how many it shows. A quiet
     country shows the mascot and "No active events in Portugal from covered sources." over its
     coverage. A code with no boundary says "We have no boundary for this place."; a failed load
     offers Retry, and a failed refresh keeps the summary with a warning. Opening a country frames
     its box once (unless the address also opens an event; a country across ±180°, such as Fiji, by
     its events, the short way round, as in the apps), focuses its name, and on a phone opens
     the sheet at half height, as in `MapScreen.swift`; the map's own strip steps aside there.
     Choosing a country in search, or a country's name in an event's header, opens its overview and
     closes the event. An event chosen from a country opens beside it and is drawn on the map even
     when the map's list doesn't hold it; the country keeps its filter and camera underneath, and
     a search carried into the Countries list ends there. Escape ends the search, then clears the
     event, then closes the country, with keyboard focus back on its row.
   - **Deviations for the owner:**
     - **Close goes up to Countries**, not back: on a shared link there may be nothing to go back
       to. Same reasoning as the event's Close.
     - **No 60% scrim** behind the phone overview: the sheet already separates it from the map.
     - **↑/↓ step the Monitor list only.** On Countries they would select from a list that isn't
       on screen.
     - **Half height on a phone**, where DESIGN §10.2 says large, so the framed country stays in view
       above its overview. The iPhone app does the same and flags it for the owner too.
     - **Choosing the open country again** (from search) frames it again and closes the event, as in
       `MapScreen.swift`; the address doesn't change, so Back doesn't see it.
   - **W5, part 3: Sources.** `SourcesView` after `SourcesView.swift` and `SourceRow.swift` at
     `/sources` (key 5): the disclaimer, "N sources", then each active source with its health dot
     and words (Healthy, Last check failed, Stale), what it covers, its last check ("Last check failed
     1 min ago · last good check 30 min ago"), "Checked every 10 min" or "Monthly release · checked
     every 6 h", licence and attribution, and Terms plus Website (Website alone without terms), then
     "Absence of data is not safety." Disabled sources are hidden. The dot follows the design
     session's rule: an error older than the last success is healthy, a newer one is a failed check,
     the server's `stale` is stale. It refreshes each time it opens; a failed refresh keeps the last
     list with "Couldn’t refresh. Showing the last list loaded.", and a failed first load offers Try
     again. On a wide screen Sources is pinned at the foot of the sidebar with the worst health of
     all sources (no dot before the first list), as in `Sidebar.swift`; on a phone it opens from a
     button beside the search field (hidden while searching) and takes the sheet at full height,
     with its title and Done pinned in the sheet's header; Done goes back where it was opened from
     (an event, a country, the sheet's height and keyboard focus), or to the map from a link. An
     on-demand source (live flights) reads "Fetched 3 h ago" and "Fetched only while the flights
     layer is shown, at most once a minute", never stale. List rows now match `EventRow.swift`: a stale source's row fades and
     says "Stale · last update 3 h ago", a resolved one says "Resolved · ended 3 h ago", and the
     source shows by name.
   - **Deviations for the owner:**
     - **A section, not a sheet.** The apps present Sources as a sheet; on the web it is a page in
       the sidebar (wide) or the sheet (phone), so it has a link, Back and key 5 like the others. At
       248 px the rows wrap more than on the Mac's 520 pt sheet. Owner verdict wanted.
     - **An open event closes on a phone** while Sources shows (the sheet would otherwise keep
       showing the event over it); Done and Back bring it back. The iPhone's Sources sheet sits over
       the event instead.
     - **Conflict rows** (W7) will need their own monthly line; a stale UCDP never marks them stale.
     - **An empty source list** reads as a failure ("Couldn’t load the sources." with Try again);
       the apps show nothing.
   - **W6, part 1: live stream.** One `/v1/stream` per browser, shared by every tab (the server
     allows 8 per IP): the tab holding the `shield-stream` Web Lock streams and relays over the
     `shield-live` BroadcastChannel; when it closes the next tab takes over from the stored cursor
     (`shield.live` v1, which only moves forward). A visible tab that hasn't heard the leader for
     45 s asks after it and, with no answer in 5 s (a frozen background tab), takes the stream;
     a throttled but running leader answers at once, since messages aren't throttled like timers.
     A tab taking over isn't live until its own `ready`. Changes apply as in `LiveMapModel.apply`: patched in place from zoom 6
     when the list isn't cut, otherwise one reload for a run of changes; more than 50 a second fold
     into one reload; `ready` and `reset` reload. An open event outside the list refetches its
     detail on a change, and one its source removes is shown as gone at once. Other views' cached
     lists are marked stale on a change, so returning to one reloads it; list and detail requests
     skip the HTTP cache (`no-cache`), so a reload the stream asks for always reaches the server. The capsule's dot fills
     while live, and "Updated now" counts the stream's heartbeat; 45 s of silence ends live. The
     60 s poll runs regardless, so a dead stream never freezes the map.
   - **W6, part 2: arrivals.** A new event the stream sends once it is `ready` (not the backlog a
     connection replays first), in view and matching the filters, fades in and grows from 0.6 over
     `arrive` (fade only under reduced motion): a copy of its marker animates over the map while the
     map's own symbol waits unseen (feature-state), and the symbol shows when the animation ends.
     One a load already drew is toasted but not animated again; past 50 a second (a source's first
     load) arrivals aren't news, and at most 20 animate at once. An arrival the map never draws (a
     reload put it in a cluster) is forgotten after 5 s. A severe or extreme one also gets the toast
     under the filters ("New · Severe earthquake · M6.1 · Chile", one line, as
     `NewEventToast.swift`): it drops in (fades under reduced motion), pressing it opens the event
     (with `choose`, so one in a cluster opens too), × dismisses it and hands focus to Filters, a
     newer one replaces it (keeping focus if it had it), and it leaves after 6 s. Screen readers
     hear each toast once from a polite live region, even the same words twice. No sound.
     - **The toast waits while pointed at or focused,** then has 6 s again; the apps don't pause
       it. A toast stays its 6 s even if the filters or the view change under it, as in the apps.
   - **Deviations for the owner:**
     - **`fetch`, not `EventSource`.** `EventSource` never surfaces the 20 s heartbeat (an `id:`-only
       block dispatches nothing) and hides a 429/503's `Retry-After`, so the stream is read with
       `fetch` and a port of `ServerSentEvents.swift`, as the apps do. Backoff as the apps: the
       server's Retry-After, else 5 s doubling to 60 s, ±20 %.
   - **W7, part 1: satellite heat detections.** `GET /v1/fires` for the map's view while the layer
     is on (off by default, saved with the filters as the apps' `showsDetections`). One switch, three
     ways in: a thermometer toggle in the map controls (tinted `detection` while on), the filter
     sheet's "Satellite heat detection" row, and `h`; the sheet's Reset turns it off. Below zoom 6
     the server's grid cells draw as soft `detection` blobs the size of their cell, scaled on a log
     scale to the busiest cell in view (`DetectionOverlay.swift`); from zoom 6 single 3–6 px dots,
     bigger with FRP, fainter at low confidence, with a thin canvas outline. The layer sits over
     event areas and under the selection ring and every marker, and never joins hazard clusters.
     Under the notices a key reads "Satellite heat detections · Detected 3 h ago", "· Zoom in for
     all" when the points were cut off, "· None in this view", or "Heat detections didn’t load"
     (in `warning`, keeping the last layer; the events' banner stays out of it). No `since` is sent:
     the server's 24 h, as the apps; the filters' window is for events. `r` reloads it too. Turned
     off and on again it starts afresh (no old view's layer meanwhile), and a failed load is
     announced once to screen readers while the key itself stays quiet as it ages.
     - **Zoom sent to the API fixed.** The web sent MapLibre's zoom rounded; the server's grid (event
       clusters' 64 px cells, heat cells) and the apps count 256 px tiles, rounded down, which is
       MapLibre's zoom + 1. Every request now sends `floor(zoom) + 1`, so clusters and heat cells
       are the screen size the server means, and points arrive from the same scale as on iPhone.
     - **Bigger blobs.** Cells are 0.9–1.4 of their width (the apps: 0.5–1). MapLibre's blur fades
       faster than the apps' gradient, and at 0.5 a burning region read as a lattice of dots, at
       the corrected zoom too (checked on live FIRMS data, desktop and phone).
     - **No "· VIIRS".** DESIGN §5's "Detected 3 h ago · VIIRS" can't come from the data (the
       API names satellites, e.g. NOAA-20, not instruments); the key follows the apps' words.
     - **No inspector for a detection.** Detections aren't selectable in the apps, so the web
       doesn't add the plan's satellite, confidence and FRP inspector (decided with W7, part 3).
   - **W7, part 2: armed conflicts.** Their own `GET /v1/events?type=conflict&kind=observed` for the
     map's view while the layer is on (off by default, saved with the filters as the apps'
     `showsConflicts`; the sheet's Reset leaves it alone, as the apps). No window: the server keeps a
     conflict 90 days after its end date. Never streamed and never in the hazards' strip, counts,
     clusters or search; checked every 6 h, a pan asks at once, `r` asks now. One switch, two ways
     in: a crosshair toggle in the map controls above heat (tinted `accent` while on) and `c`.
     Markers use the severity shape and colour like hazards, with the crosshair glyph, a dashed
     ring when the location is approximate, and are never stale; they sort under every hazard.
     Server clusters of conflicts get a dashed ring and a small crosshair badge ("Cluster, 14
     armed-conflict events, highest severity severe"). Under the notices a key reads "Armed
     conflicts, past 90 days · 12 in this view" (listed plus clustered), "· zoom in for all",
     "Armed conflicts · loading…" or "Armed conflicts didn’t load", always over "Conflict data are
     monthly research estimates (UCDP), not live reports.". The list shows them in their own section
     after the hazards ("Armed conflicts · past 90 days", with how many the map grouped); a row
     carries its dates ("24 Sep · monthly estimate (UCDP)") instead of kind, source and freshness.
     With no hazards in view the list says "No hazards in this view from covered sources." above
     the section. A conflict selects and opens like any event, and ↑ ↓ walk into the section.
     - **No category menu.** The plan opted in from a category menu; the apps have the one toggle
       (category menus are still unapproved), so the web does too.
     - **Dashed, not hollow, approximate ring.** The plan said a hollow ring labelled "Approximate
       location"; the apps draw a dashed ring and say "approximate location" in the marker's
       spoken label and the detail's location note. The web draws the dashed ring, says it in the
       detail's note, and, since canvas markers aren't read aloud, in the row's spoken name.
     - **No grouping of conflicts on the map from zoom 6.** The apps cluster nearby conflicts among
       themselves; the web draws each one (they sort under every hazard). To revisit if a busy
       region proves unreadable.
     - **Turned off with a conflict selected,** its marker goes and its detail stays, as the apps.
     - **No hazards and no conflicts** shows the hazards' empty state, not the apps' "No hazards in
       this view…" over an empty conflicts section.
     - **Cluster order.** Conflict clusters are buttons over the canvas, so they sit over hazard
       markers (the apps put them under every hazard). Hazard clusters still draw over them.
     - **Types, not `category=`.** The query names `type=conflict`, as the apps' `EventQuery`.
     - **Glyph fix (all markers).** A marker drawn first as a picture during React's render (the
       inspector's header, the toast) cached an empty glyph, leaving that type blank on the map for
       the visit; every type's glyph is now drawn once when the marker code loads. Conflicts showed it first.
   - **W7, part 3: live flights.** `GET /v1/aircraft?bbox=` for the map's view while the layer is on
     (ADR 28): off at every load and never saved, as the apps. One switch, two ways in: a plane
     toggle at the top of the map controls (tinted `accent` while on) and `a` (the apps' ⌥⌘A). Asked
     only for a view of at most four 5° cells; wider, or when the server answers 400, the key reads
     "Flights · zoom in to see them" over "Shown for an area up to about 5° across." and nothing is
     asked. Once a minute and at once on a view change, never sooner: a failure, a return to the tab
     and `r` all leave it to its minute, as the apps. A failed load for a new view drops the old
     view's positions (a later view never shows them while it loads); for the same view it keeps
     them under "The last refresh failed; these positions are older.". Aircraft draw in `ink` with a thin `canvas` edge, a plane turned
     to its track, a dot when the feed gives no track (direction not guessed); they give way to one
     another where they crowd. Under the notices, between heat and conflicts, the key reads
     "Flights · 12 in this view" (or "· none in this view", "· loading…", "Flights didn’t load"; "zoom
     in" and failures are announced, the count and age are not), the
     data's age ("Positions from 3 min ago · 11:03 UTC"), "Incomplete: part of this view has no
     recent data." when the feed says so, and "© adsb.lol contributors · ODbL 1.0". A click within
     22 px of an aircraft (a 44 px target round the apps' 28 px marker) selects it (an `accent` disc
     under it, and it wins any crowding) and opens its report in the inspector or the sheet:
     callsign (else registration, else address), emergency and military flags as the feed gives
     them, the position's age, then altitude, ground speed, track, vertical rate, type, registration,
     squawk and address in aviation units with metric after, and the apps' coverage note with the
     licence link. One selection at a time: choosing an event clears the aircraft and the other
     way round, an event arriving by the address included; Escape, Close, turning the layer off or
     zooming out clears it, and Close gives focus back to the flights switch.
     - **Not reachable by keyboard.** Aircraft are canvas symbols with no list of their own; the
       apps' VoiceOver reads each one. A keyboard-reachable list is future work.
     - **Credit in the key.** The plan put the body's `attribution` in the map's attribution
       control; it sits in the flights key instead, which shows only while the layer does.
     - **Under hazards, without avoiding them.** Aircraft draw under every hazard marker as the
       apps, but MapLibre can't make them give way to the hazards' markers (placed outside its
       collision), only to each other. A hazard under the same point wins the click.
     - **No military shape.** The plan marked military aircraft by shape; the apps say it in the
       report ("Military (as the feed flags it)"), and so does the web.
     - **Bases and historic layers** (ADR 29, 30) are in the apps' controls but not in the web plan.
     - **Shortcuts with the map focused (all).** A click on the map left focus on its canvas, where
       every shortcut was ignored, so Escape couldn't close what the click opened. The map now keeps
       only its own keys (arrows, + − =), and the others work there.
   - **W8: landing page, site header and footer.** `/` is the web's front door in place of the apps'
     three onboarding cards (owner, 2026-11-19).
     - **Hero (card 1):** the full lockup (a 576 px encode for a 224–288 px slot) on Orbital Earth
       at the apps' 45 % under a vignette that never clears to the art, so the words over it sit
       on a scrim; "Global hazard intelligence. Official sources, live." and "Open the live map".
     - **World strip:** under the hero, on a panel ("Worldwide · past 24 h"), the strip counts the
       whole world under the default filters from `/v1/events?zoom=1`, events plus everything
       clustered. Official warnings come from a second `kind=warning` answer, so they are exact
       where the map's strip says "12+". A dash while loading or failed, with the map's own words
       for why (either answer failing says so, and the reason stays while it is asked again);
       "+" when the server cut the list short; "Updated now" by this browser's clock, kept
       current. The cells filter nothing.
     - **Kinds (card 2):** "Know what you're looking at: observed, official warning, forecast,
       satellite detection." over the Sentinel beside the four kind chips.
     - **Sources:** every active source by name (linking to it) with the attribution it asks for,
       as the Sources screen lists them, and "How each source is doing". Nothing while they load
       or if they fail; no name is written into the page.
     - **Close:** "Show me the map", then the footer.
     - **Header:** a thin header with the nav mark (DESIGN.md §11) and Live map, Countries and
       Sources (each 44 px tall, the current one in `ink`). It sits on the landing page, on a page
       that isn't there and, as a bar above the sidebar and the map, on the desktop map (owner,
       2026-11-26), where it replaces the sidebar's mark. The phone map keeps its capsule and its
       whole height.
     - **Footer:** the mark and promise, the same three links, and the disclaimer.
     - **Weight:** no MapLibre and no live stream on `/`: the page and everything it imports come
       to 153 KB gzipped of JavaScript. MapLibre's stylesheet now loads with the map, not with
       every page.
     - **Head:** title and description meta, Open Graph tags, and an apple-touch-icon from the
       app icon.
     - **Settings and Alerts not in the header or footer yet.** They arrive with W9; no link goes
       nowhere meanwhile.
     - **One closing link, no "Before you start".** The apps' third card shows the disclaimer over
       "Show me the map"; the web puts the disclaimer in every page's footer instead of a second
       time above it.
     - **Footer attributions** are the sources section's (the landing page draws no map, so it has
       no basemap credit to give).
     - **`og:image` is root-relative** until the public host exists; Open Graph wants an absolute
       URL.
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

# Aegis Terra — Design

What the app should look and feel like, on every platform. `VIEWS.md` says what gets built
and in what order. `design/tokens.json` holds every value; this file explains them.
Domain rules come from `docs/DECISIONS.md` and win over anything visual here.

**Owner calls (2026-11-16):** the user-facing name is **Aegis Terra**. "Shield" stays the
codename in the repo, bundle IDs, the Go module and infra, so do not rename any of those.
The logo and the Sentinel mascot are used as they are. All five screens are specified in full,
but Saved regions and the Alert center are *built* only in Phase 3, after the 10-strangers gate.

---

## 1. Design rule

The map is the hero and trust is the product. Everything else serves those two.

1. **Colour is earned.** Red and amber mean a hazard's severity. Nothing decorative uses them.
2. **Every event says three things before anything else:** what kind of record it is
   (observed, official warning, forecast, satellite detection), who said so (source), and
   how fresh it is. If one of them is missing, the component is wrong (DECISIONS #3, #8, #11).
3. **Two severities, never merged.** The app's derived severity carries the colour. The
   source's own value (GDACS "Orange", Meteoalarm "Level 3", USGS "M6.1 · PAGER yellow")
   is printed beside it, verbatim and uncoloured (DECISIONS #2).
4. **Absence of data is not safety.** Empty places say what is and is not covered (DECISIONS #7).
5. **Calm density.** Flighty-level precision: many facts, small type, generous padding,
   nothing shouting. No alarmism in copy, motion or sound.

The direction is NASA Mission Control × Apple Weather × Flighty × a premium geospatial
intelligence platform. In practice that means a deep navy canvas, a near-black map, slate panels,
precise SF Pro type, small rounded cards, very subtle borders, floating controls, and minimal
gradients and glass.

---

## 2. Identity and voice

- **Name:** "Aegis Terra" in UI, store listing and marketing. The nav micro-label is
  `AEGIS TERRA`, with the tagline `GLOBAL HAZARD INTELLIGENCE` on onboarding and About only.
- **Voice:** a calm duty officer. It is factual, short and specific, and it gives times in relative + absolute
  form ("12 min ago · 14:02 UTC").
- **Kind words are fixed** and are used exactly like this everywhere: **Observed**,
  **Official warning**, **Forecast**, **Satellite heat detection**. A FIRMS hotspot is never
  called a "fire" or "wildfire" (DECISIONS #3).
- **Never:** "Danger!", "Stay safe!", exclamation marks, emoji, or "No threats" (say
  "No active events in this view from covered sources").
- **Permanent disclaimer** (DECISIONS #12): *"Aegis Terra does not replace your national
  emergency service. Always follow official instructions."* It sits in onboarding, the
  Sources screen, every event's footer, and the web footer.

---

## 3. Palette

All values live in `design/tokens.json`. Views use token names only, never hex.

| Token | Hex | Job |
|---|---|---|
| `canvas` | `#080E1A` | App background, the frame around the map |
| `panel` | `#111C2D` | Sheets, sidebar, inspector, cards |
| `raised` | `#203550` | Selected rows, chip fills, matrix cells |
| `ink` | `#E8EEF6` | Primary text |
| `muted` | `#9AABC2` | Metadata, source names, timestamps |
| `faint` | `#6F819A` | Placeholder, disabled, ticks (never the only carrier of meaning) |
| `hairline` | `#24324A` | 1 px borders (or `ink` at 8% alpha) |
| `accent` | `#52B6FF` | Information, links, selection, focus, primary button, user location |
| `on-accent` | `#080E1A` | Text on accent fills (never white) |
| `warning` | `#FFB54A` | Non-hazard UI warnings: stale, limited coverage, offline |
| `danger` | `#F35D62` | Errors, destructive actions |
| `ok` | `#4FD1A1` | Source "up to date" dot, "Resolved" status. Never on the map |

**Brand-only** (logo, mascot, onboarding, marketing; never UI state, severity or text):
`energy-cyan #00E9FF`, `alert-red #FF2D3D`, `armor-silver #C9CDD4`, `graphite #2B2F36`.

**Dark only for the MVP.** Every token has a `light: null` slot. A light theme is a future
design task, not something an agent fills in.

---

## 4. Severity

App severity is CAP-aligned and derived in the backend's `internal/severity`. Colour never
carries it alone: every use has **colour + shape + word**.

| Severity | Colour | Shape | Marker scale |
|---|---|---|---|
| Minor | `#8DA2BE` steel | ● circle | 0.8 |
| Moderate | `#FFB54A` amber | ◆ diamond | 1.0 |
| Severe | `#FF8540` orange | ▲ triangle | 1.15 |
| Extreme | `#F35D62` red | ⯃ octagon | 1.3 + one slow halo pulse |

- Text on a severity fill is `canvas` navy (contrast ≥ 6:1), never white.
- The `SeverityBadge` reads `▲ Severe` followed by the source value in `muted`: `▲ Severe · GDACS Orange`.
- Severity sorts every list: extreme → minor, then most recently updated.
- Amber and red appear outside severity only as `warning` and `danger` UI states, and those always come with
  an icon and words, so they cannot be read as a hazard level.

---

## 5. Kind (what sort of record this is)

Kind changes the **form** of a mark. Colour stays with severity.

| Kind | Point marker | Area on map | Chip |
|---|---|---|---|
| **Observed** | Filled shape | 18% fill + solid 1.5 px stroke | `eye` · Observed |
| **Official warning** | Filled shape + 2 px canvas ring | 10% fill + solid 2 px stroke | `checkmark.seal` · Official warning |
| **Forecast** | Outline only | No fill, dashed 1.5 px stroke `[6,4]` | `clock.arrow.circlepath` · Forecast (dashed chip border) |
| **Satellite heat detection** | 3–6 px dot, `detection #F7D774`, no glyph | none | `thermometer.high` · Satellite heat detection |

- FIRMS detections are their **own layer**, off by default and toggled from the layer
  control. They never merge into hazard clusters (DECISIONS #10).
- **Stale** (older than 3× the source's poll interval, from `/v1/sources`): the marker drops to
  45% opacity and desaturates, a small clock badge appears, and rows show `warning` text "Stale · last update 3 h ago".
- **Resolved** events leave the live map. In history and timelines they show `ok` "Resolved".

---

## 6. Hazard types

The 14 backend `type` values map to SF Symbols (Apple) and Lucide names (web) in
`tokens.json → hazardType`. The glyph sits **inside** the severity shape on the marker and
leads every row title. Verify every symbol name in the SF Symbols app at S3.2. `volcano` has
no native symbol (`mountain.2.fill` stands in), and a custom symbol is allowed later.

---

## 7. Typography

- **Apple:** SF Pro through text styles, so Dynamic Type works. Numbers that change or
  compare (counts, magnitudes, coordinates, times) use `.monospacedDigit()`. Coordinates and
  event IDs use SF Mono.
- **Web:** the `system-ui` stack. Bundle no proprietary fonts.
- **Scale** (`tokens.json → type.scale`): `statHero 34` · `title 22` · `headline 17` ·
  `body 15` · `meta 13` · `micro 11` uppercase, tracked +0.9.
- `micro` labels (`EARTH / MONITOR`, `SOURCE`, `TIMELINE`) give the mission-control feel.
  Use at most one per card.
- Numbers lead and words follow: **`2,481`** above `Observed events`.

---

## 8. Shape, material, elevation

- **Radii:** 4 (badges) · 8 (chips, rows) · 12 (cards) · 18 (largest card, floating panels).
  Sheets use the system radius. No pill-shaped cards.
- **Spacing:** 4-pt grid. Card padding is 16, sheet side gutters are 20 (iOS) and 24 (macOS/web).
- **Borders:** 1 px `hairline` on every card and floating control. Borders separate, shadows don't.
- **Material:** translucency **only** where something floats over the map (map controls, the
  inspector, the collapsed sheet): `.ultraThinMaterial` with a 40% canvas tint. Never stack
  translucent layers, never use glass on a panel that sits on the panel colour.
- **Glow:** none on UI chrome. The extreme-severity marker halo is the only glow in the app.
- **Gradients:** only the vignette on hero backgrounds (§12) and the map's atmospheric
  edge. Nothing else.

---

## 9. The map

The map is the canvas. It is near-black, low contrast and quiet, so the hazard layer is the
loudest thing on screen.

**Apple (MVP):** MapKit `MKMapView` in a representable (clustering via
`MKClusterAnnotation`, per ARCHITECTURE "Clients").
- `MKStandardMapConfiguration(elevationStyle: .flat, emphasisStyle: .muted)`, forced
  dark (`overrideUserInterfaceStyle = .dark`), `pointOfInterestFilter = .excludingAll`,
  no traffic, no 3D buildings.
- Hybrid/satellite imagery is a layer option (`MKHybridMapConfiguration`), off by default.
- The app frames the map in `canvas` chrome, so the navy identity comes from the panels, sheet and
  controls around it.
- **Honest limit:** MapKit cannot be recoloured to the navy palette. A custom navy basemap
  (MapLibre Native on Apple, or a licensed dark tile overlay) is an **open item for the
  owner** (`VIEWS.md` → Open). MapKit was the owner's call, so agents do not switch it.

**Web (Phase 2):** MapLibre GL JS with a custom dark vector style tuned to `canvas`
(water a step darker than land, admin-0 borders `hairline`, labels `faint`, no roads below
z8). Base it on a style whose **tiles and style licence allow commercial use**: check CARTO
Dark Matter's terms first, otherwise use Protomaps or OpenFreeMap. Attribution stays visible
bottom-right.

**Layers, bottom to top:** basemap → event areas (polygons, by kind §5) → FIRMS
detections → clusters → point markers → selection ring → user location (accent).

**Clusters:** a circle in `panel` with a 2 px ring in the **highest severity inside**, the count
in `ink` mono, and a severity-coloured arc segment proportional to the share at that severity.
Tap or click zooms to the cluster's extent. Server-side clustering sets the low-zoom payload
(DECISIONS #9).

**Floating controls** (top-right on iOS, top-right of the canvas on macOS/web), 44 pt hit
targets, `floating` material: layers, locate me, recentre the world, and (macOS/web) zoom ±.

---

## 10. Screens

Five primary screens, plus Filters, Sources, and Onboarding. Every screen handles **loading,
empty, stale, offline, error and no-coverage** states. They are listed under each screen.

### Platform shells

**iOS:** one full-screen map, always present.
- Top: a compact status capsule (`AEGIS TERRA` micro-label · live dot · "Updated 30 s ago")
  and below it a horizontal row of `FilterChips` (type, severity, kind, time window).
- Bottom: an interactive **sheet with three detents**:
  - *peek* (≈ 96 pt): `StatStrip` + search field.
  - *medium*: the event list for the current viewport, or the selected event / country.
  - *large*: full detail.
- The map stays interactive at peek and medium. Saved, Alerts and Sources open from the
  sheet's header buttons and Settings from the capsule, so there is no tab bar and the map keeps the screen.

**macOS:** `NavigationSplitView`.
- **Sidebar** (220–260 pt): *Monitor* (Live map), *Countries* (searchable list with active
  counts), *Saved regions*, *Alerts* (badge count), and *Sources* at the bottom with an
  overall health dot.
- **Content:** the large map canvas, with the `StatStrip` floating top-left and `FilterChips` top-centre.
- **Inspector** (`.inspector`, 320–380 pt, floating material) holds the selected event or
  country. Toggle it with ⌥⌘I.
- Toolbar: search (⌘F), time window menu, layers menu, share.

| Shortcut | Action |
|---|---|
| ⌘1–⌘5 | Map · Countries · Saved · Alerts · Sources |
| ⌘F | Search places / countries / events |
| ⌥⌘I | Toggle inspector |
| ⌘L | Open official source of selected event |
| ⌘R | Refresh now |
| ⌘0 | Fit world |
| ↑ / ↓ | Previous / next event in list |
| Esc | Clear selection |
| ⌘, | Settings |

**Web (Phase 2):** at ≥ 1024 px it mirrors macOS (left sidebar, map, right inspector). Below
1024 px it mirrors iOS (full map, bottom sheet with the same three detents). It uses the same
components, names and copy.

### 10.1 Global live map (Monitor)
*"What's happening on Earth right now, how serious, and where?"*

- **Data:** `GET /v1/events?bbox&zoom&type&severity&kind&status&since` (GeoJSON), then the
  `GET /v1/stream` SSE for `event.created|updated|resolved`, plus `GET /v1/fires` when the layer is
  on.
- **Stat strip** (`StatStrip`), three cells: **Observed events** (count in view), **High
  severity** (severe + extreme), **Official warnings**. Numbers are `statHero` mono with words
  below. Tapping a cell filters the map to that set. Next to it: the live dot + "Updated 30 s ago".
- **List** (sheet medium / macOS sidebar list when Monitor is selected) holds the events in the
  viewport, sorted by severity then recency. Each `EventRow` shows the type glyph in its severity
  shape, the title, the kind chip, the source badge and freshness.
- **New event** from SSE fades in on the map (`motion.arrive`). If it is severe or extreme and
  in view, a one-line toast appears at the top ("New · Severe earthquake · M6.1 · Chile"). There is
  no sound and no vibration (no push in the MVP).
- **States:**
  - *Loading:* the map shows immediately, with a thin accent progress line under the capsule
    and skeleton rows.
  - *Empty view:* "No active events in this view from covered sources" + `CoverageNotice`
    + "Zoom out".
  - *Stale:* the capsule reads "Last update 14 min ago" in `warning`, and the stale markers
    follow §5.
  - *Offline:* the capsule reads "Offline · showing data from 14:02", nothing is cleared, and
    there is a Retry button.
  - *Error:* an inline banner in `danger` text on `panel` with Retry. The map is never blanked.

### 10.2 Country overview
- **Entry:** country search, a tap on the country name in an event, the macOS Countries
  sidebar, or a long-press on the map → "Country overview".
- **Data:** `GET /v1/countries/{iso2}/summary`.
- **Layout (sheet large / macOS inspector, or full content pane from the Countries sidebar):**
  1. Header: the country name (`title`), ISO code mono, and "Updated 2 min ago". Map fits the country
     bounds and dims everything outside (scrim 60%).
  2. `StatStrip`: Active · Severe+ · Official warnings.
  3. `TypeSeverityMatrix`: rows are hazard types present, columns minor → extreme. Cells are
     `raised` with a count, tinted by severity at 25% when non-zero, and tapping one filters
     the list.
  4. `CoverageNotice` (always shown when coverage is partial): "Official warnings for
     Portugal come from Meteoalarm. Earthquakes: USGS, GDACS. Not covered: national flood
     service." The phrase "Absence of data is not safety" is shown once, in `muted`.
  5. Recent changes (last 24 h, from revisions): "Wildfire near Leiria raised to Severe ·
     40 min ago".
  6. Event list (same `EventRow`).
- **States:** *No active events* shows the mascot `EmptyState` + the coverage notice. *Unknown
  country* shows "We have no boundary for this place."

### 10.3 Event detail
- **Data:** `GET /v1/events/{id}` (detail + revisions + countries + source).
- **Layout** (sheet large on iOS, inspector on macOS, right panel on web):
  1. **Header band:** the type glyph in its severity shape, then the title (`title`) and
     place + countries. On iOS large, a background matches the type, at 30% under a vignette
     (§12).
  2. **Severity row:** `SeverityBadge` (app) **and** the verbatim source value side by side,
     under the micro-label `SEVERITY · AEGIS / SOURCE`.
  3. **Kind + status:** `KindChip` · status (Active / Resolved) · started, observed and
     updated times (`meta` mono, relative + UTC).
  4. **Key figures** (type-specific, from `attrs`): earthquake → magnitude (statHero),
     depth, PAGER. Cyclone → category, max wind, pressure. Wildfire → area, FRP. Flood →
     affected area. Show only what exists, and never invent a figure.
  5. **Map snippet** of the event geometry by kind (§5), with a "Show on map" button.
  6. **Source block:** `SourceBadge` (name, licence, attribution), fetched time, and the primary
     button **Open official source** (accent, ⌘L).
  7. `RevisionTimeline`: vertical, newest first, with each change in words ("Severity Moderate →
     Severe", "Area grew", "Status resolved") and its time.
  8. Footer: the disclaimer (§2), plus a Share link.
- **States:** *Stale* shows a `warning` banner at the top. *Resolved* shows a "Resolved 3 h ago" `ok`
  chip, and the header desaturates. *Gone (404)* shows "This event is no longer published by its
  source" with a link to the source.

### 10.4 Saved regions (Phase 3 build)
Build only after the 10-strangers gate and Phase 3 accounts. The spec is complete now.

- **Data (Phase 3 API, not in v1):** saved regions (country or drawn bbox/polygon) with
  per-region filters, synced via Sign in with Apple.
- **List:** a `SavedRegionCard` per region, with a 2:1 mini-map thumbnail (`radius.md`), the name, an
  "Active: 4 · Severe+: 1" strip, the highest severity as a `SeverityBadge` top-right (no
  coloured left-edge bars), and the last change ("Flood warning issued · 1 h ago"). Sorting is by highest severity, then name.
- **Add region:** draw a rectangle or polygon on the map, or pick a country. Then name it and
  pick filters (types, minimum severity, kinds). The default is official warnings + observed,
  severity ≥ moderate, with detections off.
- **Detail:** reuses the Country overview layout (§10.2) scoped to the region's geometry.
- **Empty:** the Sentinel mascot `EmptyState`, "Watch the places that matter to you", and an
  Add region button.
- On iOS it opens from the sheet header's bookmark button. On macOS it is a sidebar item.

### 10.5 Alert center (Phase 3 build)
Build after the gate, with APNs push. The spec is complete now.

- **Data (Phase 3 API):** the alerts generated for the user's saved regions.
- Each `AlertRow` distinguishes the three CAP axes as separate, labelled fields. Never fold them into
  one colour:
  - **Severity**, as a `SeverityBadge` (colour + shape + word).
  - **Urgency**: Immediate / Expected / Future / Past. Plain text, with a clock icon.
  - **Certainty**: Observed / Likely / Possible / Unlikely. Plain text, with a confidence icon.
  - Plus the kind chip, region name, source and time.
- Rows are grouped Today / Yesterday / Earlier. Unread rows have an accent dot. Swipe to mark read.
  There is no "dismiss all" for extreme alerts.
- Tapping a row opens the Event detail.
- **Notification preferences** (in the region detail): per-region minimum severity, kinds, quiet
  hours (extreme severity can bypass them, opt-in). Defaults are conservative, with no alerts below
  severe.
- **Empty:** "No alerts. You'll see official warnings and observed events for your saved regions here."

### 10.6 Filter sheet
- Type (multi-select grid of the 14 glyphs), severity (minimum, as a 4-step segmented control with
  shapes), kind (four toggles, *Satellite heat detection* off by default), time window (1 h ·
  6 h · 24 h · 7 d), and status (Active / include resolved).
- Filters persist locally (S3.3). Active filters show as removable chips on the map. "Reset"
  appears when anything differs from the default.

### 10.7 Sources & attribution
- **Data:** `GET /v1/sources`.
- `SourceRow` per source: name, what it covers, licence, attribution text, poll interval,
  last success ("2 min ago", with an `ok` dot) or last error (`warning`/`danger` dot + time).
- The disclaimer at the top. A link to each source's terms.
- This screen must list every active source before external TestFlight (BACKLOG S3.7, M4).

### 10.8 Onboarding (first run, 3 cards, skippable)
1. The full logo lockup on the Orbital Earth background, with "Global hazard intelligence. Official
   sources, live."
2. The Sentinel mascot beside the four kind chips: "Know what you're looking at: observed,
   official warning, forecast, satellite detection."
3. The disclaimer, with a "Show me the map" button.

There is no account and no permission prompt in the MVP. Location permission is asked only when the user
taps Locate.

---

## 11. Brand art (owner: used as-is)

Source files are in `design/brand/` (see its README).

- **App icon:** the shield-and-globe portion of the logo, **without the wordmark**, centred on
  opaque `canvas #080E1A`, exported at 1024². iOS icons cannot be transparent, and the
  wordmark is illegible at icon sizes. This is a crop of the as-is art, not a redraw. The owner
  signs off on the crop.
- **Full lockup** (shield + "AEGIS TERRA" wordmark) appears on onboarding, About, the macOS About
  window, the web landing hero, marketing and store screenshots.
- **Nav mark:** the same shield crop at 20–28 pt in the iOS status capsule, the macOS sidebar
  header and the web header.
- **Sentinel mascot:** onboarding card 2, `EmptyState` (no events in view, no saved regions,
  no alerts), About, and marketing. Version B is the canonical pose. Show it at most once per
  screen, never smaller than 120 pt tall.
- **Never on the map canvas, inside an event or alert, or next to a severity.** The art's
  chrome reds and cyans must not compete with hazard markers. This is the asset pack's own rule.
- Clear the trademark for "Aegis Terra" and confirm the AI-art licence before App Store submission
  (M4).

---

## 12. Backgrounds

There are four original SVGs in `design/brand/backgrounds/` (1920×1080). They are used under a
radial vignette to `canvas` and kept low contrast, never behind body text without a panel.

| File | Used for |
|---|---|
| `01_Orbital_Earth` | Onboarding hero, About, web landing hero |
| `02_Radar_Grid` | Loading skeleton backdrop, empty states |
| `03_Precipitation_Atmosphere` | Event detail header for storm, tropical cyclone, heavy rain, flood, wind |
| `04_Wildfire_Contours` | Event detail header for wildfire, heat, drought, volcano |

Other types get a plain `panel` header. Satellite or photo backgrounds are Phase 5 and need licensing.

---

## 13. Motion

Values are in `tokens.json → motion`. Motion explains a change and never decorates.

- Sheet detents and the inspector use `spring` (0.35 s, damping 0.86).
- Cluster split and merge use `standard`, with markers moving from or to the cluster centre.
- A new event (SSE) uses `arrive`, fading and scaling from 0.6.
- The extreme halo uses `extremePulse`: one ring, every 1.8 s, only while the marker is on screen.
- **Reduce Motion:** every transition becomes a cross-fade and the pulse becomes a static
  double ring.

---

## 14. Accessibility

- Every colour pair is in `tokens.json → contrastPairs`, and the token generator fails CI below its
  minimum (AA: 4.5 text, 3.0 graphics).
- Severity is never colour-only (§4). Kind is never colour-only (§5).
- **VoiceOver** label per marker: "Severe earthquake, magnitude 6.1, observed, 12 minutes ago,
  source USGS." Clusters: "Cluster, 14 events, highest severity extreme." The map list is the
  accessible alternative to the map and is always reachable.
- Dynamic Type up to AX5 in sheets and lists. The stat strip wraps to two lines and never truncates numbers.
- Hit targets are ≥ 44 pt. Focus rings use `accent`. macOS has full keyboard navigation (§10 shortcuts).
- With Increase Contrast, `hairline` becomes `hairlineStrong` and material becomes opaque `panel`.

---

## 15. Data honesty

- Any non-live number (previews, screenshots, mockups, demo mode) carries a visible
  **Sample data** badge (`SampleDataBadge`). Mock data is never shown as a live warning.
- Freshness is visible on every screen (capsule, rows, detail).
- Every event links to its official source.
- The `countries` from geometry can list more than one country ("Portugal, Spain"), so never pick one.
- Store screenshots use real data captured with permission and attribution, or are badged as
  sample.

---

## 16. Visual references (steal tone, not layout)

Links are in `design/brand/source/Inspiration_Links_and_Credits.md`. Do not copy layouts,
tiles, imagery or trademarks pixel for pixel.

| Look at | Take this |
|---|---|
| **Flighty** | Information hierarchy, compact dark cards, numbers-first rows, restrained colour |
| **Apple Weather (iOS / macOS)** | Native sidebar, translucent sections over a map, the luminous precipitation layer on a dark basemap |
| **Watch Duty** | Incident boundaries, actionable event detail, source + update timeline |
| **Fuselab "Earthquake Prediction Data"** | macOS / web density: incident feed + map + stat panels side by side |
| **Ventusky** | Cinematic globe overview and a bottom time scrubber (Phase 5 timeline replay) |
| **Zoom Earth / Windy** | Layer picker and severity gradients (Phase 5 layers) |
| **NASA Black Marble** | A future "night" basemap idea |
| **CARTO Dark Matter** | How quiet a basemap must be for overlays to read (web, licence-checked) |

---

## 17. Do not

- Use red or amber for anything that isn't a hazard severity or an explicit warning/error state.
- Call a FIRMS detection a fire or wildfire, or cluster detections with hazards.
- Merge, average or recolour the source's severity. Show both.
- Show an empty map as "safe". Show coverage instead.
- Put the logo or mascot on the map, inside an event or alert, or next to a severity.
- Stack glass, add neon glow to chrome, or use gradients beyond the vignette.
- Add decorative charts. A chart exists only if it answers a question the screen asks.
- Use oversized rounded cards, pill-shaped cards, or a generic SaaS dashboard grid.
- Put hex literals or `Color(red:)` in views. Add a token.
- Copy third-party UI, map tiles, imagery or logos.
- Show sample numbers without the Sample data badge.
- Build Saved regions or the Alert center before the Phase 3 gate.
- Rename anything in code or infra from "shield". The product name is UI-only.

---

## Next docs

- `VIEWS.md`: surface map, component catalog with props and endpoints, build order.
- `design/tokens.json`: every value. S3.1 copies `apps/aaru/scripts/generate-tokens.mjs`
  and `ci-brand.yml` to generate `Tokens.gen.swift`, colour sets and web CSS from it.
- `design/brand/README.md`: the asset inventory and licence notes.
- `design/mockups/README.md`: the clickable mockup canvas for owner verdicts (live link + sources).

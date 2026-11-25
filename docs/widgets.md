# Widgets: plan (draft, 2026-11-25)

Status: **plan only, nothing built.** The owner asked (2026-11-25) to "at least start planning for
some widgets" on iOS and macOS. Everything below marked *proposal* is Claude's call, for the owner
to correct; nothing here is decided until the owner says so.

## 1. What a glance should answer

*Proposal.* A Shield widget answers one question without opening the app: **"Is anything serious
happening in the place I watch, and how fresh is that?"** One place, chosen by the person: a
country first (the API already has a per-country overview), later a point with a radius ("near
me"). Not the world: a worldwide count is large every day and says nothing at a glance.

What a widget must never do (`DESIGN.md` §5 and §15 at the repo root, ADR 22):

- show sample data as if it were live, or a number without its age ("Updated 12 min ago");
- call a FIRMS detection a wildfire, a base "active", or a conflict estimate "live";
- spend the flights feed's quota: no flights widget;
- invent a severity: the counts are the server's, by severity, as in the country overview.

## 2. Candidate widgets, in order

| # | Widget | Sizes | Data | Tap opens |
|---|--------|-------|------|-----------|
| W1 | **Watched country** | iOS small, medium; macOS small, medium | `GET /v1/countries/{iso2}/summary` (`CountrySummary`: counts per severity, coverage, latest events, `generatedAt`) | that country's overview |
| W2 | **Lock Screen** (iOS) | accessoryInline ("2 severe · Portugal"), accessoryCircular (highest severity glyph and count) | same as W1 | same |
| W3 | **Near me** | small, medium | `GET /v1/events?bbox=` around the location, by severity | the map there |
| W4 | **Followed event** (Live Activity) | Lock Screen, Dynamic Island | an event's updates | the event |

Left out on purpose: flights (quota), bases and historic sites (static; nothing changes at a
glance), heat detections alone (a count of hotspots without context misleads).

Small (W1): the country's name, the highest severity present as its marker shape and colour, the
active count, and "Updated N min ago". Medium adds the two most severe events' titles and whether
an official warning feed covers the country (`coverage`).

```
┌──────────────────────┐   ┌───────────────────────────────────────────────┐
│ PORTUGAL             │   │ PORTUGAL                    ◆ 3 severe · 11   │
│ ◆ Severe             │   │ ◆ Orange warning for wind · Faro              │
│ 11 active            │   │ ● Wildfire in Leiria (observed)               │
│ Updated 12 min ago   │   │ Official warnings: IPMA · Updated 12 min ago  │
└──────────────────────┘   └───────────────────────────────────────────────┘
```

## 3. How it would be built

- **A WidgetKit extension** in `shield-apple`, one target for iOS and macOS, using `shield-core`
  and `shield-api` as the app does. Its timeline provider calls the API itself; the summary →
  entry logic (which severity leads, what the "Updated" line says, when to show "No data") lives
  in `shield-core`, unit-tested like `LiveMapModel`.
- **Configuration through App Intents, not an App Group.** The country is a parameter of the widget
  (`WidgetConfigurationIntent` with a country `AppEntity` whose query reads `/v1/countries`), set in
  the widget's own edit sheet. That needs no data shared with the app, so no App Group, no
  entitlement change and no new portal capability for W1 and W2. An App Group (to follow the app's
  saved filters or a "watched" list) is a later choice and the owner's: it needs a capability on
  the Apple Developer portal and Khepri's fastlane profiles updated.
- **Refresh:** a timeline entry every 30 minutes (`.after`), well inside WidgetKit's daily reload
  budget; the API's own caching absorbs it. A failed fetch keeps the last entry and says how old
  it is, as the map does; it never blanks the widget.
- **Tap targets:** a URL scheme (`aegisterra://country/PT`, `aegisterra://event/<id>`) handled by
  `onOpenURL` in the app, which has no deep links today. This is new app surface and needs its own
  small step (and tests for the parsing).
- **W3 "Near me"** adds location in the widget (`NSWidgetWantsLocation`, the person's location
  permission) and a radius; a privacy note goes with it.
- **W4 Live Activity** needs push updates (APNs) to stay current, which is infrastructure: the
  owner's call under "product first", and after the web launch.

## 4. What blocks a real widget today

1. **No API host until the web launch (ADR 22).** Beta and Release builds have no `API_BASE_URL`,
   so a widget there would show sample data, which it must label as such. Debug builds point at
   the local API, so W1 can be built and checked in the simulator and on this Mac now.
2. **Signing:** a new extension target needs its own bundle ID
   (`com.fernandocorreia.shield.beta.widgets`) and provisioning profile in Khepri's fastlane
   setup. The owner decides when (no accounts or infrastructure until the owner says so).
3. **Strangers count is 0:** widgets are supply-side polish; the owner's rule is to flag that.

## 5. Proposed order

1. W1 Watched country, Debug only, against the local API: extension target, App Intent config,
   small and medium, iOS and macOS, entry logic tested in `shield-core`.
2. The URL scheme and `onOpenURL` for the tap.
3. W2 Lock Screen accessories (iOS).
4. After the web launch: Beta/Release builds with the API host, signing for the extension.
5. Later: W3 near me, W4 Live Activity (push).

## 6. Open for the owner

- Is "the place I watch" the right question, or is it something else (a hazard type, the world's
  extremes)?
- Country first, or "near me" first?
- The URL scheme's name (`aegisterra://` follows the UI name; the bundle is `shield`).
- When to add the extension's signing to Khepri's fastlane.

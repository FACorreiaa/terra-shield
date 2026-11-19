# Severity mapping

Shield stores two severities on every event and never merges them (ADR 2):

- `source_severity`: the source's own words, verbatim enough to recognize ("M5.4 mww, PAGER green").
- `severity`: Shield's CAP-aligned level, `minor < moderate < severe < extreme`, derived in
  `backend/internal/severity`. The clients color by this one and show the source's value beside it.

Every row below is a case in `internal/severity/severity_test.go`. Change a table here and there together.

## Earthquakes (USGS)

The highest of three signals wins.

| Signal | minor | moderate | severe | extreme |
|---|---|---|---|---|
| Magnitude | < 5.0 | 5.0–5.9 | 6.0–6.9 | ≥ 7.0 |
| PAGER alert (when issued) | green | yellow | orange | red |
| Tsunami flag | — | floor | — | — |

- **PAGER** is USGS's estimate of fatalities and economic loss. It reflects impact on people, so it can lift a
  moderate quake under a dense, vulnerable city. A green PAGER never lowers a large magnitude.
- **Tsunami flag** (`tsunami=1`) means tsunami information may exist for a large oceanic event, not that a
  tsunami happened. It floors the level at moderate. Official tsunami warnings come from their own sources.
- Events without a magnitude are skipped until USGS assigns one.

## GDACS (cyclones, floods, droughts, wildfires, volcanoes)

| GDACS alert | Shield | Why |
|---|---|---|
| Green | minor | low humanitarian impact, local response |
| Orange | severe | medium impact; international assistance may be needed |
| Red | extreme | high impact; international assistance likely needed |

- GDACS alerts estimate humanitarian impact (hazard × exposed people × vulnerability), not physical size.
- **Physical size floors the alert** (owner, 2026-11-19), as magnitude does for earthquakes: the level is the
  higher of the alert and the size GDACS reports in `gdacs:severity`. Without this, a Category 4 hurricane
  over open ocean (Green) showed as minor next to an Orange 65 km/h storm near the coast shown as severe.
  - **Tropical cyclones**, maximum wind (km/h) on the Saffir-Simpson bands:

    | Wind | Floor |
    |---|---|
    | < 119 (below hurricane force) | none: the alert decides |
    | 119–177 (Category 1–2) | moderate |
    | 178–251 (Category 3–4) | severe |
    | ≥ 252 (Category 5) | extreme |

    The value is GDACS's structured one; its label can disagree ("Tropical Storm (maximum wind speed of
    176 km/h)"), and the number wins.
  - **Wildfires**, burned area (ha) on the wildfire table below, shared with every fire source.
  - Floods, droughts and volcanoes have no size field and follow the alert alone.
- `attrs.severityBasis` says what decided: `alert`, `wind` or `area`. An `area` level is also
  `severityProvisional` (owner, 2026-11-19), like every EONET fire: area says nothing about the people nearby.
- Footprints (S1.3b) are still fetched only for Orange and Red alerts, not for events lifted by a floor.
- The placeholder "Magnitude 0" that GDACS sends for floods is dropped from `sourceSeverity` ("Green alert").
- GDACS earthquakes are not ingested; USGS (above) owns earthquakes.

## Wildfires: one burned-area table (EONET's IRWIN and BC Wildfire; floor under GDACS)

| Burned area | Shield |
|---|---|
| < 1,000 acres (NWCG classes A–E) | minor |
| 1,000–9,999 acres | moderate |
| 10,000–99,999 acres | severe |
| ≥ 100,000 acres ("megafire") | extreme |

Hectares convert at 2.471 acres/ha. `sourceSeverity` reads "2,379 acres burned" / "5,000 ha burned". Every EONET event carries `severityProvisional: true` (attrs now; a top-level API field from S2.1). **Provisional:** area says nothing about the people nearby. Other EONET
categories (volcanoes, landslides, dust, snow) carry no severity and show as minor.

- **Prescribed burns** (IRWIN titles "Prescribed Fire … RX") stay on the map so someone who sees smoke can find
  the cause, but are always **minor**, with `attrs.prescribed: true` (owner, 2026-11-19): a planned, controlled
  fire is not a hazard of its size. On 2026-11-19 they were 16 of 24 active EONET wildfires.

## Armed conflicts (UCDP; ADR 27, owner 2026-11-19)

| Best fatality estimate | Shield | September 2026 share |
|---|---|---|
| 0–4 | minor | 87% |
| 5–24 | moderate | 11% |
| 25–99 | severe | 1.5% |
| ≥ 100 | extreme | 0.5% |

`sourceSeverity` reads "28 deaths (3–31)" (best, then UCDP's low–high range) or "7 deaths". The count is a research estimate and the whole basis of the level. `severityProvisional` is set when UCDP's `code_status` says the deaths are still being checked, the source is vague or biased, or another check is open (plain "Check"). Conflicts are not hazards: they never count toward a country's hazard severity.

## FIRMS

Detections, not events: no severity. FRP (fire radiative power) and confidence are stored with each detection.

## Meteoalarm

MeteoAlarm's awareness level is the one color scale every member shares, so it is used rather than CAP `severity`,
which members fill differently (DWD sends `Minor` for yellow; MeteoAlarm's own label for yellow is `Moderate`).

| `awareness_level` | Shield |
|---|---|
| 1 green | not a warning: skipped |
| 2 yellow | moderate |
| 3 orange | severe |
| 4 red | extreme |

`source_severity` shows "Yellow warning" / "Orange warning" / "Red warning". The CAP `severity`, `certainty` and
`urgency` are kept in attrs.

## Tsunami (NOAA warning centers; owner, 2026-11-19)

US products (Hawaii, Alaska, the US West Coast, Guam, American Samoa, the Marianas, Puerto Rico, the Virgin
Islands) are official alerts for the NWS zones they name, rated by level:

| Alert | Shield | NWS meaning |
|---|---|---|
| Tsunami Warning | extreme | dangerous coastal flooding and currents: move inland |
| Tsunami Advisory | severe | strong currents and waves: stay off beaches |
| Tsunami Watch | moderate | a tsunami is possible: be ready to act |
| Information, Cancellation | not stored; ends the product's event | |

CAP's own `severity` rates Advisory and Watch the same (Moderate), so the level is used instead. One message can hold
segments at several levels along different coasts; each level is its own event (ADR 25).

PTWC's threat messages for the Pacific and Caribbean are forecasts for national authorities (`kind=forecast`,
`attrs.guidance`), rated by the highest wave band that names a coast:

| Forecast | Shield |
|---|---|
| waves over 3 m | extreme |
| 1–3 m | severe |
| 0.3–1 m | moderate |
| under 0.3 m only, or "the threat has passed" | not stored; ends the forecast |
| "hazardous waves possible" (first message, before a forecast is computed) | **severe** (provisional: not in the owner's table) |

`attrs.forecast` keeps every band with its places and countries. Countries come from the names PTWC lists
(Hawaii → US, Chuuk → FM, Kermadec Islands → NZ), not from the marker, which sits at the epicenter.

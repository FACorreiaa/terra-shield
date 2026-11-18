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
  A 176 km/h tropical storm over open ocean is Green, so it shows as minor. Its wind speed is still shown
  in `sourceSeverity`.
- GDACS has no level between Green and Orange, so it never yields moderate.
- GDACS earthquakes are not ingested; USGS (above) owns earthquakes.

## Wildfires without an alert level (EONET: IRWIN, BC Wildfire, ...)

| Burned area | Shield |
|---|---|
| < 1,000 acres (NWCG classes A–E) | minor |
| 1,000–9,999 acres | moderate |
| 10,000–99,999 acres | severe |
| ≥ 100,000 acres ("megafire") | extreme |

Hectares convert at 2.471 acres/ha. `sourceSeverity` reads "2,379 acres burned" / "5,000 ha burned". Every EONET event carries `severityProvisional: true` (attrs now; a top-level API field from S2.1). **Provisional:** area says nothing about the people nearby. Other EONET
categories (volcanoes, landslides, dust, snow) carry no severity and show as minor.

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

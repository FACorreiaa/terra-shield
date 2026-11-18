# Event `attrs`, per source

`Event.attrs` in `/v1/events` holds type- and source-specific fields. Every key is optional unless marked
**always**. Keys are camelCase. Clients must ignore keys they do not know, because sources can add keys
without a new API version.

The API lifts `attrs.severityProvisional` to the top-level `severityProvisional` field and removes it from
`attrs`.

## USGS (earthquakes)

| Key | Type | Meaning |
|---|---|---|
| `mag` | number, **always** | Magnitude |
| `magType` | string, **always** | Magnitude type (`mww`, `ml`, `md`, …) |
| `place` | string, **always** | USGS's place text ("10 km SSW of Ridgecrest, CA") |
| `sig` | integer, **always** | USGS significance, 0–1000+ |
| `tsunami` | boolean, **always** | USGS tsunami flag: a tsunami is *possible*, not observed |
| `reviewStatus` | string, **always** | `automatic` or `reviewed` |
| `network` | string, **always** | Contributing network (`us`, `ci`, `ak`, …) |
| `depthKm` | number | Hypocenter depth |
| `pagerAlert` | string | PAGER impact alert: `green` · `yellow` · `orange` · `red` |
| `feltReports` | integer | "Did You Feel It?" responses |
| `cdi` | number | Community decimal intensity |
| `mmi` | number | ShakeMap maximum intensity |

## GDACS (cyclones, floods, droughts, volcanoes, wildfires)

| Key | Type | Meaning |
|---|---|---|
| `alertLevel` | string, **always** | `Green` · `Orange` · `Red` |
| `eventId` | string, **always** | GDACS event id |
| `episodeId` | string, **always** | GDACS episode id (bumps when the event is re-assessed) |
| `point` | [lon, lat], **always** | GDACS's reference position |
| `countries` | string[], **always** | GDACS's own country names. For display only: country matching uses geometry (ADR 4) |
| `footprint` | boolean | `true` when `geometry` is GDACS's impact polygon rather than the point |
| `alertScore` | number | GDACS alert score |
| `severityText` | string | GDACS's severity sentence |
| `severityValue` | number | Raw severity value, unit in `severityUnit` (e.g. km/h for cyclones) |
| `severityUnit` | string | |
| `populationText` | string | Affected-population sentence |
| `vulnerability` | string | GDACS vulnerability class |
| `eventName` | string | Storm name or GDACS event name |
| `glide` | string | GLIDE disaster number |
| `bbox` | [minLon, minLat, maxLon, maxLat] | GDACS bounding box |
| `capUrl` | string | GDACS CAP message |

## NASA EONET (mostly incident-level wildfires)

| Key | Type | Meaning |
|---|---|---|
| `severityProvisional` | boolean, **always** `true` | Lifted to the top level by the API |
| `category` | string, **always** | EONET category id (`wildfires`, `volcanoes`, …) |
| `upstream` | string[], **always** | Agencies EONET took it from (`IRWIN`, `SIVolcano`, …) |
| `observations` | integer, **always** | Number of EONET observations so far |
| `eonetUrl` | string, **always** | The EONET record |
| `magnitudeValue` | number | Latest magnitude: burned area for wildfires |
| `magnitudeUnit` | string | `acres` · `hectare` · … |

## Meteoalarm (EU weather warnings; not live yet, see BACKLOG S1.5)

| Key | Type | Meaning |
|---|---|---|
| `awarenessLevel` | integer, **always** | 2 yellow · 3 orange · 4 red |
| `awarenessType` | string, **always** | MeteoAlarm hazard label (`Wind`, `snow-ice`, `Thunderstorm`, …) |
| `capSeverity` | string, **always** | CAP `severity` as the member sent it |
| `certainty` | string, **always** | CAP certainty |
| `urgency` | string, **always** | CAP urgency |
| `expiresAt` | date-time, **always** | When the warning expires |
| `sender` | string, **always** | National service ("Deutscher Wetterdienst") |
| `language` | string, **always** | Language of `title`/`summary` (English when the member provides it) |
| `areas` | string[], **always** | Warning area names |
| `emmaIds` | string[], **always** | MeteoAlarm area codes |
| `country` | string, **always** | ISO2 of the member feed |

## FIRMS

FIRMS detections are not events. They are served by `/v1/fires`, with properties `satellite`, `acquiredAt`,
`frp`, `confidence` and `iso2`.

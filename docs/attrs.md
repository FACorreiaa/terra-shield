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

## Meteoalarm (EU weather warnings)

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

## Tsunami (NOAA warning centers; ADR 25)

Two kinds of event: US alerts (`kind=warning`, id `<center>-<incident>-<product>-warning|advisory|watch`, one event
per alert level of a product) and PTWC threat forecasts (`kind=forecast`, id `<center>-<incident>-<product>`).

Both kinds:

| Key | Type | Meaning |
|---|---|---|
| `center` | string, **always** | `PTWC` (Pacific, Honolulu) or `NTWC` (National, Palmer, Alaska) |
| `product` | string, **always** | WMO product id: `WEHW40` Hawaii, `WEGM40` Guam, `WEAK51` US West Coast/Alaska, `WEPA40` Pacific threat, … |
| `bulletin` | integer, **always** | Message number within the incident and product |
| `incident` | string, **always** | The center's incident id |
| `epicenter` | [lon, lat], **always** | Earthquake epicenter |
| `geometry` | string, **always** | `zones` (drawn from NWS zones) or `epicenter` (a point: forecasts always, a US alert whose zones could not be drawn) |
| `expiresAt` | date-time, **always** | US: the CAP expiry, renewed by each message. Forecasts: 12 h after the last message (safety net) |
| `magnitude` | number | Preliminary magnitude |
| `magnitudeType` | string | Magnitude type (`Mwp`, …) |

US alerts (`kind=warning`):

| Key | Type | Meaning |
|---|---|---|
| `zones` | string[], **always** | NWS zone codes (UGC) the alert names, marine and Canadian ones included |
| `areas` | string[], **always** | One description per coastal segment ("Chignik Bay, Alaska to Samalga Pass, Alaska"); can be long |
| `zonesUnresolved` | string[] | Named zones with no shape in the NWS zone file (marine `PZZ…`, Canadian `BCZ…`, renumbered zones) |

Threat forecasts (`kind=forecast`):

| Key | Type | Meaning |
|---|---|---|
| `guidance` | boolean, **always** | `true`: guidance for national authorities, not a warning. Show it as such |
| `forecast` | object[], **always** | Bands, most severe first: `{band, severity, places: string[], countries: string[]}`. `band` is `over 3 m`, `1–3 m`, `0.3–1 m` or `hazardous waves possible` (first message, before heights exist; rated severe provisionally) |
| `unmatchedPlaces` | string[] | Places PTWC named that map to no country code |

The event's countries come from the zones (US alerts) or the bands (forecasts), not from the geometry: a
forecast's point marks the epicenter, not where waves are expected.

## FIRMS

FIRMS detections are not events. They are served by `/v1/fires`, with properties `satellite`, `acquiredAt`,
`frp`, `confidence` and `iso2`.

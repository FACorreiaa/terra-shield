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

## EMSC (earthquakes in Europe and the Mediterranean; ADR 34)

Names follow USGS's where they mean the same. No `sig`, `tsunami`, `reviewStatus` or PAGER keys.

| Key | Type | Meaning |
|---|---|---|
| `mag` | number, **always** | Magnitude |
| `magType` | string, **always** | Magnitude type (`ml`, `mb`, `mw`, `m`, …) |
| `place` | string, **always** | Flynn-Engdahl region, title case ("Dodecanese Islands, Greece") |
| `depthKm` | number | Hypocenter depth |
| `agency` | string | The network that located it (`INGV`, `NOA`, `AFAD`, `EMSC`, …) |

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

## National Weather Service (US official warnings; ADR 31)

One event is one NWS hazard event (its P-VTEC code); the latest message speaks for it. `startedAt` is the VTEC begin
when NWS gives one, else the first message Shield saw: once an event is under way NWS sends each message's own time
as its onset, so a stored start is kept.

| Key | Type | Meaning |
|---|---|---|
| `office` | string, **always** | Issuing office (`KPQR` Portland, `KWNS` Storm Prediction Center) |
| `phenomena` | string, **always** | VTEC phenomenon code (`TO` tornado, `WS` winter storm, `CF` coastal flood, …) |
| `significance` | string, **always** | VTEC significance: `W` warning, `A` watch, `Y` advisory, `S` statement |
| `etn` | string, **always** | Event tracking number (per office, phenomenon and significance; restarts each year) |
| `event` | string, **always** | NWS product name ("Winter Storm Warning") |
| `capSeverity` | string, **always** | CAP `severity` of the segment that set the level (`Unknown` when NWS gave none; the level then follows the significance) |
| `certainty` | string, **always** | CAP certainty of the latest message |
| `urgency` | string, **always** | CAP urgency of the latest message |
| `sender` | string, **always** | Issuing office's name ("NWS Portland OR") |
| `areas` | string[], **always** | Area names of every segment in force, sorted |
| `endsAt` | date-time | When the event is expected to end (CAP `ends`, else the VTEC end); not an expiry, the event stays while NWS lists it |

## NHC cyclone forecasts (ADR 32)

One event is one storm. The pin (`centroid`) is the storm's center from the latest public advisory; the geometry is the
latest forecast cone. Winds are knots, sustained over one minute.

| Key | Type | Meaning |
|---|---|---|
| `stormId` | string, **always** | NHC storm id: basin, number, year (`al052025`) |
| `basin` | string, **always** | `AL` Atlantic, `EP` eastern North Pacific, `CP` central North Pacific (CPHC) |
| `classification` | string, **always** | NHC code: `TD`, `STD`, `TS`, `STS`, `HU`, `PTC` (post-tropical), `PC` (potential), … |
| `advisory` | string | Latest public advisory number, intermediate ones included (`011A`) |
| `forecastAdvisory` | string, **always** | Advisory number of the forecast the cone and track come from (`011`) |
| `forecastIssuedAt` | date-time, **always** | When that forecast was issued |
| `position` | [lon, lat], **always** | The storm's center, as `centroid` |
| `windKt` | number | Maximum sustained wind now |
| `pressureMb` | number | Minimum central pressure now |
| `movementDeg`, `movementMph` | number | Direction the storm is moving toward, and its speed |
| `peakWindKt` | number, **always** | Strongest wind in the forecast (rates severity) |
| `forecast` | object[], **always** | The track: `tau` (hours after the forecast's synoptic time), `at`, `lon`, `lat`, `windKt`, `gustKt`, `pressureMb` (initial point only), `stormType` (`TD`, `TS`, `HU`, `MH` major hurricane, `LO` remnant low, …); unknown values are left out |

## USGS volcano alert levels (ADR 35)

One event is one US volcano while it is above normal, keyed by its GVP volcano number (`usgs:<code>` for the few GVP
does not number). `started_at` is when the current level began; `updated_at` the latest notice.

| Key | Type | Meaning |
|---|---|---|
| `alertLevel` | string, **always** | Ground alert level: `ADVISORY`, `WATCH`, `WARNING`, or `UNASSIGNED` |
| `colorCode` | string, **always** | Aviation colour code: `YELLOW`, `ORANGE`, `RED`, or `UNASSIGNED` |
| `alertLevelPrev`, `colorCodePrev` | string | The levels before the current ones |
| `observatory` | string, **always** | `avo` Alaska, `hvo` Hawaiian, `cvo` Cascades, `calvo` California, `yvo` Yellowstone, `nmi` Northern Mariana Islands |
| `vnum` | string | GVP volcano number (the join key with EONET's volcanoes) |
| `volcanoCode` | string | USGS volcano code (`hi3`) |
| `volcanoUrl` | string | The observatory's page for the volcano |
| `region` | string | USGS region (`Aleutians`, `Hawaii`) |
| `noticeId` | string | The latest notice; the event's `url` opens it |
| `threat` | string | USGS national threat ranking (`Very High Threat` … `Very Low Threat`), a standing assessment, not today's activity |

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

## UCDP (armed conflicts; ADR 27)

A conflict event is one incident of organized lethal violence, from UCDP's monthly Candidate release: a research estimate, one to two months old, shown for 90 days after its end date. `observedAt`, `updatedAt` and `startedAt` are the event's own dates (UCDP gives no edit time): show "Sep 21" and "monthly estimate (UCDP)", not "updated N ago". News-source text and names are never stored.

| Key | Type | Meaning |
|---|---|---|
| `violenceType` | string, **always** | `state_based` (a government is a side) · `non_state` (organized groups) · `one_sided` (an actor against civilians) |
| `conflictName` | string, **always** | UCDP's conflict name ("Pakistan: Government") |
| `sideA`, `sideB` | string, **always** | The two sides; `Civilians` for one-sided violence. "Unidentified group" where UCDP cannot name the actor. Names are UCDP's own, kept verbatim ("Government of Russia (Soviet Union)", "Yemen (North Yemen)"; owner, 2026-11-19) |
| `deathsBest`, `deathsLow`, `deathsHigh` | integer, **always** | Fatality estimates; `deathsBest` sets the severity |
| `deathsSideA`, `deathsSideB`, `deathsCivilians`, `deathsUnknown` | integer, **always** | The best estimate split by who died |
| `locationPrecision` | string, **always** | `exact` · `within_25km` · `admin2` (district centroid) · `admin1` (province centroid) · `area` (a line or fuzzy area). Draw anything but `exact` as approximate. Country-level and at-sea events are not ingested |
| `datePrecision` | string, **always** | `day` · `days_2_6` · `week` · `month` · `months` (up to a year): `startedAt`–`observedAt` is the range |
| `codeStatus` | string, **always** | UCDP's review status: `Clear`, or `Check deaths`, `Check dyad`, `Check geography`, … |
| `sourceCount` | integer, **always** | How many sources UCDP used |
| `expiresAt` | RFC 3339, **always** | 90 days after the end date; the event resolves then |
| `severityProvisional` | boolean | Lifted to the top level by the API |

## FIRMS

FIRMS detections are not events. They are served by `/v1/fires`, with properties `satellite`, `acquiredAt`,
`frp`, `confidence` and `iso2`.

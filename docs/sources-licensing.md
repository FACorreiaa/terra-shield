# Sources: licensing, cadence, limits

Checked 2026-11-16 against each source's live endpoint. Shown in-app on the Sources & attribution screen
(S3.7) from `/v1/sources`, whose license, attribution, coverage line, terms page (`termsUrl`) and publishing
cadence come from each adapter's `Info()`. A terms page is listed only once verified live: GDACS
(`termofuse.aspx`), NASA EONET and FIRMS (Earthdata's data use guidance), NOAA (`weather.gov/disclaimer`) and UCDP
(the download page) were checked on 2026-11-19. USGS's policy page refuses scripted requests and MeteoAlarm's terms
document has not been located, so neither has a `termsUrl` yet. adsb.lol (live flights) is listed too, registered by the API rather than the worker, with `onDemand: true`: it is fetched only while someone views the layer.

| Source | Feed used | License | Attribution | Publishes | Shield polls | Conditional GET |
|---|---|---|---|---|---|---|
| USGS | `earthquake.usgs.gov/.../summary/2.5_day.geojson` | Public domain (U.S. Government work) | U.S. Geological Survey | every minute | 60 s | ignored; Last-Modified compared client-side |
| GDACS | `www.gdacs.org/xml/rss.xml` (+ `gdacsapi/api/polygons/getgeometry` for Orange/Red) | CC BY 4.0 (European Union) | GDACS (UN + European Commission) | every few minutes | 5 min | ETag → 304 |
| NASA EONET | `eonet.gsfc.nasa.gov/api/v3/events?status=all&days=30` | Public domain (NASA) | NASA EONET | minutes to days, per upstream | 10 min | none (store skips unchanged) |
| NASA FIRMS | `firms.modaps.eosdis.nasa.gov/data/active_fire/*-viirs-c2/csv/*_Global_24h.csv` | NASA open data, no restrictions | NASA FIRMS | per satellite pass (~3 h NRT latency) | 15 min | ignored; HEAD + ETag |
| Meteoalarm | `feeds.meteoalarm.org/api/v1/warnings/feeds-<country>` | CC BY 4.0 | MeteoAlarm (EUMETNET) | per national service | 5 min (planned) | none sent |
| NOAA Tsunami Warning Centers | `www.tsunami.gov/events/js/previous.js` (last 40 messages) + each message's CAP and bulletin text | Public domain (U.S. Government work) | NOAA/NWS Pacific and National Tsunami Warning Centers | on each earthquake that could cause a tsunami; updates about hourly | 60 s | Last-Modified |
| UCDP Candidate Events | `ucdp.uu.se/downloads/candidateged/GEDEvent_v<YY>_0_<M>.csv`, the months that can hold an event of the last 90 days (no token; the API needs one) | CC BY 4.0, citation required: Hegre, Croicu, Eck and Högbladh (2020), "Introducing the UCDP Candidate Events Dataset", *Research & Politics* | Uppsala Conflict Data Program | monthly, around the 20th, for the month before | 6 h | none (each file read in full; the store skips unchanged) |
| adsb.lol (live flights, ADR 28) | `api.adsb.lol/v2/point/<lat>/<lon>/<NM>` per 5° cell, on demand from the API | ODbL 1.0 (share-alike on any database published from it; Shield publishes none, positions are cached for seconds) | "© adsb.lol contributors", in every `/v1/aircraft` answer | continuously (positions under 60 s old) | at most once a minute per cell, 450 a day (one every 3.2 min after a burst of 12) | none |
| Wikidata (military bases, ADR 29) | `query.wikidata.org/sparql`, one SPARQL query per installation class (16) | CC0 1.0 (no attribution required; Shield credits it anyway) | "Wikidata", in every `/v1/bases` answer, and a link to each item | edited continuously by volunteers | weekly, all 16 classes in one poll | none (the layer is replaced whole) |
| Wikidata (historic sites, ADR 30) | `query.wikidata.org/sparql`, one SPARQL query per class (119, plus the 16 base classes) and one for World Heritage Sites | CC0 1.0 (no attribution required; Shield credits it anyway) | "Wikidata", in every `/v1/historic` answer, and a link to each item | edited continuously by volunteers | weekly | none (the layer is replaced whole) |
| NWS forecast zones | `www.weather.gov/source/gis/Shapefiles/WSOM/z_16ap26.zip` (pinned, SHA-256) | Public domain (U.S. Government work) | National Weather Service | when zones change | loaded by `migrate` | — |

Notes:

- **GDACS** requires credit: "reuse is allowed, provided appropriate credit to GDACS is given".
- **EONET** re-publishes other agencies' events; each event links to its upstream (IRWIN, Smithsonian GVP, ...).
  Upstreams Shield ingests directly are dropped (ADR 5).
- **FIRMS** public 24 h files need no MAP_KEY. The MAP_KEY API (area queries, archives) is not used.
  Suomi NPP's file was empty on 2026-11-16 (last modified 2026-11-12); NOAA-20 and NOAA-21 carry ~80k rows/day each.
- **Meteoalarm** warning areas arrive as inline CAP polygons or as codes. Codes are drawn from MeteoAlarm's own
  geocodes file and alias list (Redistribution Hub, `gitlab.com/meteoalarm-pm-group/documents`, CC BY 4.0, pinned
  to the 2026-11-13 commit; no token). Its terms add one condition to CC BY: modified information must be
  redistributed alongside the unmodified original. Shield keeps the source's own severity verbatim next to the
  derived one (ADR 2) and links every warning to MeteoAlarm. National members may add their own terms; check
  before the external TestFlight.
- **Tsunami** (ADR 25): the Atom feeds (`PHEBAtom.xml`, `PAAQAtom.xml`) hold only each center's latest message, and
  a center sends several products per round (Kamchatka 2025: Hawaii, Guam, American Samoa and the Pacific threat
  message within minutes), so Shield reads `previous.js`, the data file behind tsunami.gov's own list of recent
  messages. It is not a documented API: if its format changes the poll fails and the staleness alert fires. Only
  paths on tsunami.gov's events tree are fetched from it. PTWC's international threat messages say they are "issued
  for information only … meant for national authorities"; Shield stores them as forecasts, labelled as guidance,
  and never counts them as official-warning coverage.
- **Excluded** (non-commercial terms): Open-Meteo, RainViewer (ADR 11).

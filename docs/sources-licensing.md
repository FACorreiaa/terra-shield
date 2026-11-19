# Sources: licensing, cadence, limits

Checked 2026-11-16 against each source's live endpoint. Shown in-app on the Sources & attribution screen
(S3.7) from `/v1/sources`, whose license and attribution come from each adapter's `Info()`.

| Source | Feed used | License | Attribution | Publishes | Shield polls | Conditional GET |
|---|---|---|---|---|---|---|
| USGS | `earthquake.usgs.gov/.../summary/2.5_day.geojson` | Public domain (U.S. Government work) | U.S. Geological Survey | every minute | 60 s | ignored; Last-Modified compared client-side |
| GDACS | `www.gdacs.org/xml/rss.xml` (+ `gdacsapi/api/polygons/getgeometry` for Orange/Red) | CC BY 4.0 (European Union) | GDACS (UN + European Commission) | every few minutes | 5 min | ETag → 304 |
| NASA EONET | `eonet.gsfc.nasa.gov/api/v3/events?status=all&days=30` | Public domain (NASA) | NASA EONET | minutes to days, per upstream | 10 min | none (store skips unchanged) |
| NASA FIRMS | `firms.modaps.eosdis.nasa.gov/data/active_fire/*-viirs-c2/csv/*_Global_24h.csv` | NASA open data, no restrictions | NASA FIRMS | per satellite pass (~3 h NRT latency) | 15 min | ignored; HEAD + ETag |
| Meteoalarm | `feeds.meteoalarm.org/api/v1/warnings/feeds-<country>` | CC BY 4.0 | MeteoAlarm (EUMETNET) | per national service | 5 min (planned) | none sent |
| NOAA Tsunami Warning Centers | `www.tsunami.gov/events/js/previous.js` (last 40 messages) + each message's CAP and bulletin text | Public domain (U.S. Government work) | NOAA/NWS Pacific and National Tsunami Warning Centers | on each earthquake that could cause a tsunami; updates about hourly | 60 s | Last-Modified |
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

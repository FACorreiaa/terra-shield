/**
 * What the map shows, as the API understands it, and the cluster arc. Mirrors shield-core
 * `MapViewport` and `EventCluster.severityShares` (MapTypes.swift), and the server's rules for
 * `bbox` (`features/params`) and the live flights grid (`internal/aircraft`).
 */
import type { EventCluster, Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";

import { wrapLongitude } from "./geo.ts";

/** A map's visible bounds as MapLibre gives them: longitudes may run past ±180 on a wrapped map. */
interface Bounds {
  west: number;
  south: number;
  east: number;
  north: number;
}

/** The query a map view sends: an integer zoom and `bbox` as `minLon,minLat,maxLon,maxLat`. */
interface View {
  zoom: number;
  bbox: string;
}

interface SeverityShare {
  severity: Severity;
  share: number;
}

const HALF_TURN = 180;
const FULL_TURN = 360;
const QUARTER_TURN = 90;
/** The view's latitudes stop here, inside Web Mercator's limit. */
const VIEW_LATITUDE_LIMIT = 85;
/** Two decimals: about 1 km, enough for a view and stable as a query key. */
const BBOX_SCALE = 100;
/** The API's zoom range (`params.MaxZoom`). */
const MAX_ZOOM = 22;
const WORLD_BBOX = `${-HALF_TURN},${-VIEW_LATITUDE_LIMIT},${HALF_TURN},${VIEW_LATITUDE_LIMIT}`;
/** The live flights cache grid, in degrees (`aircraft.CellDegrees`). */
const AIRCRAFT_CELL_DEGREES = 5;
/** Cells one `/v1/aircraft` request may span (`aircraft.Options.MaxCells` default). */
const AIRCRAFT_MAX_CELLS = 4;
const LAT_CELLS = HALF_TURN / AIRCRAFT_CELL_DEGREES;
const LON_CELLS = FULL_TURN / AIRCRAFT_CELL_DEGREES;
const BBOX_VALUES = 4;

function rounded(value: number): number {
  // `+ 0` turns -0 into 0, so a query never reads "-0".
  return Math.round(value * BBOX_SCALE) / BBOX_SCALE + 0;
}

function clamp(value: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(value, hi));
}

/**
 * The API's zoom for a MapLibre zoom: a whole number in 0…22, 0 when unknown. The server's pixel
 * grid (event clusters' 64 px cells, heat detection cells) assumes 256 px tiles, as the apps'
 * `MapViewport.zoom` does, while MapLibre's zoom counts 512 px tiles: one level less for the same
 * scale. Rounded down, as the apps, so a server cell never covers less screen than it assumes.
 */
function apiZoom(zoom: number): number {
  return Number.isFinite(zoom) ? clamp(Math.floor(zoom) + 1, 0, MAX_ZOOM) : 0;
}

/**
 * A view's west and east edges in [-180, 180], two decimals. A west edge on 180 or an east edge
 * on -180 is the same meridian on the side the view is on, unless moving both would turn a narrow
 * view at ±180° into the whole band.
 */
function edges(west: number, east: number): [west: number, east: number] {
  const minLon = rounded(wrapLongitude(west));
  const maxLon = rounded(wrapLongitude(east));
  const movedWest = minLon === HALF_TURN ? -HALF_TURN : minLon;
  const movedEast = maxLon === -HALF_TURN ? HALF_TURN : maxLon;
  return movedWest === -HALF_TURN && movedEast === HALF_TURN ? [minLon, maxLon] : [movedWest, movedEast];
}

function latitude(lat: number): number {
  return rounded(clamp(lat, -VIEW_LATITUDE_LIMIT, VIEW_LATITUDE_LIMIT));
}

/**
 * The query for a map view. A view 180° or more across (the globe zoomed out), or bounds that
 * aren't numbers, ask for the whole world. Longitudes are brought into [-180, 180]; a view across
 * the antimeridian then has `west > east`, which `/v1/events` reads as crossing (ARCHITECTURE §4).
 */
function viewOf(bounds: Bounds, zoom: number): View {
  const view = { zoom: apiZoom(zoom), bbox: WORLD_BBOX };
  const { west, south, east, north } = bounds;
  if (![west, south, east, north].every((value) => Number.isFinite(value)) || south > north) {
    return view;
  }
  const span = east >= west ? east - west : east + FULL_TURN - west;
  if (span >= HALF_TURN) {
    return view;
  }
  const [minLon, maxLon] = edges(west, east);
  return { ...view, bbox: [minLon, latitude(south), maxLon, latitude(north)].join(",") };
}

/** A `bbox` the server accepts (`params.BBox`), as four numbers; undefined when it would 400. */
function parseBBox(bbox: string): [number, number, number, number] | undefined {
  const parts = bbox.split(",").map((part) => part.trim());
  if (parts.length !== BBOX_VALUES || parts.some((part) => part === "")) {
    return undefined;
  }
  const [minLon = Number.NaN, minLat = Number.NaN, maxLon = Number.NaN, maxLat = Number.NaN] = parts.map(Number);
  const lonOk = [minLon, maxLon].every((lon) => lon >= -HALF_TURN && lon <= HALF_TURN);
  const latOk = minLat >= -QUARTER_TURN && maxLat <= QUARTER_TURN && minLat <= maxLat;
  return lonOk && latOk ? [minLon, minLat, maxLon, maxLat] : undefined;
}

function latIndex(lat: number): number {
  return clamp(Math.floor((lat + QUARTER_TURN) / AIRCRAFT_CELL_DEGREES), 0, LAT_CELLS - 1);
}

function lonIndex(lon: number): number {
  return clamp(Math.floor((lon + HALF_TURN) / AIRCRAFT_CELL_DEGREES), 0, LON_CELLS - 1);
}

/** How many 5° cells `/v1/aircraft` would load for `bbox`, counted as the server counts them. */
function aircraftCells(bbox: string): number | undefined {
  const box = parseBBox(bbox);
  if (box === undefined) {
    return undefined;
  }
  const [minLon, minLat, maxLon, maxLat] = box;
  const rows = latIndex(maxLat) - latIndex(minLat) + 1;
  const columns =
    minLon > maxLon ? LON_CELLS - lonIndex(minLon) + lonIndex(maxLon) + 1 : lonIndex(maxLon) - lonIndex(minLon) + 1;
  return rows * columns;
}

/** Whether `/v1/aircraft` serves this `bbox`: a valid box within four cells, so never a 400. */
function aircraftAllowed(bbox: string): boolean {
  const cells = aircraftCells(bbox);
  return cells !== undefined && cells <= AIRCRAFT_MAX_CELLS;
}

/**
 * Share of each severity, most severe first, for a cluster's arc (DESIGN.md §9). Empty when the
 * server sent no split: the marker then shows the maximum only, never a guessed split.
 */
function severityShares(cluster: Pick<EventCluster, "severities">): SeverityShare[] {
  const total = Object.values(cluster.severities).reduce((sum, count) => sum + count, 0);
  if (total <= 0) {
    return [];
  }
  return SEVERITY_ORDER.toReversed().flatMap((severity) => {
    const count = cluster.severities[severity];
    return count === undefined ? [] : [{ severity, share: count / total }];
  });
}

/** MapLibre's globe at zoom 0 is a 512 px world wrapped round a sphere: 512 / π across at the equator. */
const WORLD_PIXELS_AT_Z0 = 512;
const GLOBE_DIAMETER_AT_Z0 = WORLD_PIXELS_AT_Z0 / Math.PI;
/** Share of the limiting side the globe fills: most of a phone's width, less of a wide screen's height. */
const GLOBE_FILL = { portrait: 0.9, landscape: 0.62 } as const;
const GLOBE_ZOOM = { min: 0.5, max: 3 } as const;
const DEGREES_TO_RADIANS = Math.PI / HALF_TURN;

interface Size {
  width: number;
  height: number;
}

/**
 * The starting zoom that shows the whole globe in a map of this size, centred on `centerLatitude`.
 * MapLibre scales the globe by cos(latitude) so the centre matches the flat map; this undoes that.
 */
function globeZoom({ width, height }: Size, centerLatitude: number): number {
  const fill = width < height ? GLOBE_FILL.portrait : GLOBE_FILL.landscape;
  const target = Math.min(width, height) * fill;
  const atZ0 = GLOBE_DIAMETER_AT_Z0 * Math.cos(centerLatitude * DEGREES_TO_RADIANS);
  if (!(target > 0) || !(atZ0 > 0)) {
    return GLOBE_ZOOM.min;
  }
  return clamp(Math.log2(target / atZ0), GLOBE_ZOOM.min, GLOBE_ZOOM.max);
}

/** The flat map's zoom that fits the world's width into `width` pixels (MapLibre's world is 512 px at z0). */
function flatWorldZoom(width: number): number {
  return width > 0 ? Math.max(0, Math.log2(width / WORLD_PIXELS_AT_Z0)) : 0;
}

export type { Bounds, SeverityShare, Size, View };
export {
  AIRCRAFT_MAX_CELLS,
  aircraftAllowed,
  aircraftCells,
  apiZoom,
  flatWorldZoom,
  globeZoom,
  severityShares,
  viewOf,
};
// The box helpers, where the brief for this module put them.
export type { GeoBox } from "./geo.ts";
export { contains, crossesAntimeridian, geoBox, longitudeSpan, unwrapped, WORLD, wrapLongitude } from "./geo.ts";

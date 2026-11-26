/**
 * Boxes, rings and shapes on a longitude/latitude map. Mirrors shield-core `GeoBox`
 * (MapTypes.swift), `EventGeometry` and its `frame` (EventDetail.swift).
 */
import type { components } from "@shield/api-client";

type Coordinate = components["schemas"]["Coordinate"];
type Geometry = components["schemas"]["Geometry"];

/** A box in degrees. `west > east` means it crosses ±180°, the API's `bbox` convention. */
interface GeoBox {
  west: number;
  south: number;
  east: number;
  north: number;
}

/** An event's footprint: a point is already the marker; lines and polygons are drawn. */
type EventShape =
  | { kind: "point"; point: Coordinate }
  | { kind: "lines"; lines: Coordinate[][] }
  /** Each polygon is its rings: the outer boundary, then holes. */
  | { kind: "polygons"; polygons: Coordinate[][][] };

const HALF_TURN = 180;
const FULL_TURN = 360;
/** Web Mercator stops here; a box past it asks for latitudes no map can show. */
const MERCATOR_LATITUDE_LIMIT = 85.05112878;
/** A framed shape is padded by a fifth of its extent on each side. */
const FRAME_PADDING = 1.4;
/** A framed shape is at least this many degrees across. */
const MINIMUM_FRAME_SPAN = 2;
const BBOX_VALUES = 4;
const WEST = 0;
const SOUTH = 1;
const EAST = 2;
const NORTH = 3;

const WORLD: GeoBox = {
  west: -HALF_TURN,
  south: -MERCATOR_LATITUDE_LIMIT,
  east: HALF_TURN,
  north: MERCATOR_LATITUDE_LIMIT,
};

/** The wire's `[minLon, minLat, maxLon, maxLat]` as a box; undefined when it is not four numbers. */
function geoBox(bbox: readonly number[]): GeoBox | undefined {
  const [west, south, east, north] = [bbox[WEST], bbox[SOUTH], bbox[EAST], bbox[NORTH]];
  if (
    bbox.length !== BBOX_VALUES ||
    west === undefined ||
    south === undefined ||
    east === undefined ||
    north === undefined
  ) {
    return undefined;
  }
  return { west, south, east, north };
}

/** Whether the box crosses ±180°. */
function crossesAntimeridian(box: GeoBox): boolean {
  return box.west > box.east;
}

/** Width in degrees of longitude, across the antimeridian when the box crosses it. */
function longitudeSpan(box: GeoBox): number {
  return crossesAntimeridian(box) ? box.east + FULL_TURN - box.west : box.east - box.west;
}

/** Whether the box holds the coordinate, either side of ±180° when it crosses. */
function contains(box: GeoBox, coordinate: Coordinate): boolean {
  if (coordinate.latitude < box.south || coordinate.latitude > box.north) {
    return false;
  }
  return crossesAntimeridian(box)
    ? coordinate.longitude >= box.west || coordinate.longitude <= box.east
    : coordinate.longitude >= box.west && coordinate.longitude <= box.east;
}

/** A longitude in [-180, 180]; ±180 themselves are kept. */
function wrapLongitude(longitude: number): number {
  let wrapped = longitude % FULL_TURN;
  if (wrapped > HALF_TURN) {
    wrapped -= FULL_TURN;
  }
  if (wrapped < -HALF_TURN) {
    wrapped += FULL_TURN;
  }
  return wrapped;
}

/** `longitude` moved by whole turns to within 180° of `previous`. */
function near(longitude: number, previous: number): number {
  let lon = longitude;
  while (lon - previous > HALF_TURN) {
    lon -= FULL_TURN;
  }
  while (previous - lon > HALF_TURN) {
    lon += FULL_TURN;
  }
  return lon;
}

/**
 * `ring` with continuous longitudes (each step under 180°), so a shape crossing ±180° stays one
 * shape: [170, -170] becomes [170, 190]. With `anchor`, the ring starts within 180° of it, which
 * lines a polygon's holes up with its outer ring.
 */
function unwrapped(ring: readonly Coordinate[], anchor?: number): Coordinate[] {
  const [first] = ring;
  if (first === undefined) {
    return [];
  }
  let previous = anchor === undefined ? first.longitude : near(first.longitude, anchor);
  return ring.map((coordinate, index) => {
    const longitude = index === 0 ? previous : near(coordinate.longitude, previous);
    previous = longitude;
    return { latitude: coordinate.latitude, longitude };
  });
}

/** `[lon, lat]` → coordinate. */
function position(value: unknown): Coordinate | undefined {
  if (!Array.isArray(value) || value.length < 2) {
    return undefined;
  }
  const [longitude, latitude] = value as unknown[];
  if (typeof longitude !== "number" || typeof latitude !== "number") {
    return undefined;
  }
  return { latitude, longitude };
}

/** Every element must map, or the whole list is rejected (no half-drawn shapes). */
function list<Item>(value: unknown, map: (item: unknown) => Item | undefined): Item[] | undefined {
  if (!Array.isArray(value) || value.length === 0) {
    return undefined;
  }
  const out: Item[] = [];
  for (const item of value as unknown[]) {
    const mapped = map(item);
    if (mapped === undefined) {
      return undefined;
    }
    out.push(mapped);
  }
  return out;
}

/** A ring or line string: at least two positions. */
function line(value: unknown): Coordinate[] | undefined {
  const points = list(value, position);
  return points !== undefined && points.length >= 2 ? points : undefined;
}

/**
 * The GeoJSON geometry of `GET /v1/events/{id}` as a shape; undefined when the coordinates don't
 * have the shape the type promises, and for a MultiPoint (a point set has no area).
 */
function eventShape(geometry: Geometry): EventShape | undefined {
  const raw = geometry.coordinates;
  switch (geometry.type) {
    case "Point": {
      const point = position(raw);
      return point && { kind: "point", point };
    }
    case "MultiPoint": {
      return undefined;
    }
    case "LineString": {
      const ring = line(raw);
      return ring && { kind: "lines", lines: [ring] };
    }
    case "MultiLineString": {
      const lines = list(raw, line);
      return lines && { kind: "lines", lines };
    }
    case "Polygon": {
      const rings = list(raw, line);
      return rings && { kind: "polygons", polygons: [rings] };
    }
    case "MultiPolygon": {
      const polygons = list(raw, (polygon) => list(polygon, line));
      return polygons && { kind: "polygons", polygons };
    }
    default: {
      return undefined;
    }
  }
}

/** Every ring or line of a shape; a point is a ring of one. */
function ringsOf(shape: EventShape): Coordinate[][] {
  if (shape.kind === "point") {
    return [[shape.point]];
  }
  return shape.kind === "lines" ? shape.lines : shape.polygons.flat();
}

/** The values' extent, padded by a fifth on each side and at least `minimumSpan` across. */
function padded(values: readonly number[], minimumSpan: number): [number, number] {
  // A loop, not Math.min(...values): a large shape's vertices would overflow the call stack.
  let lo = Number.POSITIVE_INFINITY;
  let hi = Number.NEGATIVE_INFINITY;
  for (const value of values) {
    lo = Math.min(lo, value);
    hi = Math.max(hi, value);
  }
  const span = Math.max(hi - lo, minimumSpan / FRAME_PADDING);
  const mid = (lo + hi) / 2;
  const half = (span * FRAME_PADDING) / 2;
  return [mid - half, mid + half];
}

/**
 * The box that frames a shape on the map ("Show on map"): its extent padded by a fifth on each
 * side, at least 2° across, one box even across ±180°. A point gets the minimum.
 */
function frame(shape: EventShape, centroid: Coordinate, minimumSpan: number = MINIMUM_FRAME_SPAN): GeoBox {
  const points = ringsOf(shape).flatMap((ring) => unwrapped(ring, centroid.longitude));
  if (points.length === 0) {
    return frame({ kind: "point", point: centroid }, centroid);
  }
  const [west, east] = padded(
    points.map((point) => point.longitude),
    minimumSpan,
  );
  const [south, north] = padded(
    points.map((point) => point.latitude),
    minimumSpan,
  );
  return {
    west: wrapLongitude(west),
    south: Math.max(south, -MERCATOR_LATITUDE_LIMIT),
    east: wrapLongitude(east),
    north: Math.min(north, MERCATOR_LATITUDE_LIMIT),
  };
}

export type { Coordinate, EventShape, GeoBox };
export {
  contains,
  crossesAntimeridian,
  eventShape,
  frame,
  geoBox,
  longitudeSpan,
  MERCATOR_LATITUDE_LIMIT,
  unwrapped,
  WORLD,
  wrapLongitude,
};

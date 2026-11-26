import type { components } from "@shield/api-client";

import { relative } from "./freshness.ts";

type FireCollection = components["schemas"]["FireFeatureCollection"];

/** Detections counted per grid cell (below zoom 6). */
interface DetectionCell {
  longitude: number;
  latitude: number;
  count: number;
  /** Highest fire radiative power in the cell, MW. */
  maxFrp: number | undefined;
  latestAt: Date;
}

/** One satellite heat detection (from zoom 6). */
interface Detection {
  longitude: number;
  latitude: number;
  /** e.g. "NOAA-20". */
  satellite: string;
  acquiredAt: Date;
  /** Fire radiative power, MW. */
  frp: number | undefined;
  confidence: "low" | "nominal" | "high" | undefined;
}

/**
 * NASA FIRMS satellite thermal detections (`GET /v1/fires`, `Detections.swift`). Never called
 * "wildfires" (ADR 10, DESIGN.md §5): a detection is heat seen from orbit, about 3 h after the pass.
 */
type DetectionLayer = (
  | { kind: "grid"; cells: readonly DetectionCell[]; cellDegrees: number }
  | { kind: "points"; points: readonly Detection[]; truncated: boolean }
) & {
  /** The newest acquisition, for "Detected 3 h ago"; found once, when the answer is read. */
  latest: Date | undefined;
};

const EMPTY_DETECTIONS: DetectionLayer = { kind: "grid", cells: [], cellDegrees: 0, latest: undefined };

type FireFeature = FireCollection["features"][number];

/** `[longitude, latitude]` of a GeoJSON point, or undefined for any other geometry. */
function pointOf(geometry: FireFeature["geometry"]): [number, number] | undefined {
  const coordinates: unknown = geometry.coordinates;
  if (geometry.type !== "Point" || !Array.isArray(coordinates)) {
    return undefined;
  }
  const longitude: unknown = coordinates[0];
  const latitude: unknown = coordinates[1];
  return typeof longitude === "number" && typeof latitude === "number" ? [longitude, latitude] : undefined;
}

function validDate(text: string): Date | undefined {
  const date = new Date(text);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function cellOf({ geometry, properties }: FireFeature): DetectionCell | undefined {
  const point = pointOf(geometry);
  const latestAt = "count" in properties ? validDate(properties.latestAt) : undefined;
  if (point === undefined || latestAt === undefined || !("count" in properties)) {
    return undefined;
  }
  const [longitude, latitude] = point;
  return { longitude, latitude, count: properties.count, maxFrp: properties.maxFrp, latestAt };
}

function detectionOf({ geometry, properties }: FireFeature): Detection | undefined {
  const point = pointOf(geometry);
  const acquiredAt = "satellite" in properties ? validDate(properties.acquiredAt) : undefined;
  if (point === undefined || acquiredAt === undefined || !("satellite" in properties)) {
    return undefined;
  }
  const [longitude, latitude] = point;
  const { satellite, frp, confidence } = properties;
  return { longitude, latitude, satellite, acquiredAt, frp, confidence };
}

function defined<Item>(item: Item | undefined): item is Item {
  return item !== undefined;
}

/**
 * The layer a `/v1/fires` answer describes. The collection's `aggregation` says what its features
 * are (the two property shapes carry no tag); a feature that isn't a usable point is skipped.
 */
/** The latest of `dates`, by a loop: a spread of tens of thousands of arguments can throw. */
function newest(dates: readonly Date[]): Date | undefined {
  let [found] = dates;
  for (const date of dates) {
    if (found !== undefined && date > found) {
      found = date;
    }
  }
  return found;
}

function detectionLayer(collection: FireCollection): DetectionLayer {
  if (collection.aggregation === "grid") {
    const cells = collection.features.map((feature) => cellOf(feature)).filter((cell) => defined(cell));
    const latest = newest(cells.map((cell) => cell.latestAt));
    return { kind: "grid", cells, cellDegrees: collection.cellDegrees ?? 0, latest };
  }
  const points = collection.features.map((feature) => detectionOf(feature)).filter((point) => defined(point));
  const latest = newest(points.map((point) => point.acquiredAt));
  return { kind: "points", points, truncated: collection.truncated, latest };
}

function sameCell(one: DetectionCell, other: DetectionCell | undefined): boolean {
  return (
    other !== undefined &&
    one.longitude === other.longitude &&
    one.latitude === other.latitude &&
    one.count === other.count &&
    one.maxFrp === other.maxFrp &&
    one.latestAt.getTime() === other.latestAt.getTime()
  );
}

function samePoint(one: Detection, other: Detection | undefined): boolean {
  return (
    other !== undefined &&
    one.longitude === other.longitude &&
    one.latitude === other.latitude &&
    one.frp === other.frp &&
    one.confidence === other.confidence &&
    one.acquiredAt.getTime() === other.acquiredAt.getTime()
  );
}

/**
 * Whether two answers draw the same: FIRMS changes every 15 min at most, so most minutely reloads
 * match, and the map needn't take tens of thousands of features again.
 */
function sameDetections(one: DetectionLayer, other: DetectionLayer): boolean {
  if (one.kind === "grid" && other.kind === "grid") {
    return (
      one.cellDegrees === other.cellDegrees &&
      one.cells.length === other.cells.length &&
      one.cells.every((cell, index) => sameCell(cell, other.cells[index]))
    );
  }
  if (one.kind === "points" && other.kind === "points") {
    return (
      one.truncated === other.truncated &&
      one.points.length === other.points.length &&
      one.points.every((point, index) => samePoint(point, other.points[index]))
    );
  }
  return false;
}

interface LegendState {
  failed: boolean;
  loading: boolean;
  now: Date;
}

/**
 * What the heat layer is and how old it is (`DetectionLegend`, MapScreen.swift): "Detected 3 h ago",
 * never "Updated", since FIRMS lags the satellite pass by about 3 h (DESIGN.md §5).
 */
function detectionLegend(layer: DetectionLayer, { failed, loading, now }: LegendState): string {
  if (failed) {
    return "Heat detections didn’t load";
  }
  const parts = ["Satellite heat detections"];
  const { latest } = layer;
  if (latest !== undefined) {
    parts.push(`Detected ${relative(latest, now)}`);
  } else if (loading) {
    parts.push("Loading…");
  } else {
    parts.push("None in this view");
  }
  if (layer.kind === "points" && layer.truncated) {
    parts.push("Zoom in for all");
  }
  return parts.join(" · ");
}

export type { Detection, DetectionCell, DetectionLayer };
export { detectionLayer, detectionLegend, EMPTY_DETECTIONS, sameDetections };

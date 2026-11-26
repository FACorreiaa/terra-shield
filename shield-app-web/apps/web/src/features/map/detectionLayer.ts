import type { Feature, FeatureCollection, Point } from "geojson";
import type { CircleLayerSpecification, ExpressionSpecification } from "maplibre-gl";

import { colorTokens, kindColors } from "@/design/tokens.gen.ts";
import type { DetectionLayer } from "@/domain/detections.ts";

/**
 * Satellite heat detections (DESIGN.md §5, §9; DetectionOverlay.swift): a `detection`-tinted density
 * below zoom 6, single 3–6 px dots from 6. Its own layer, under the selection ring and every marker,
 * never merged into hazard clusters (DECISIONS #10).
 */

const DETECTIONS_SOURCE = "detections";
const DETECTION_CELLS = "detection-cells";
const DETECTION_DOTS = "detection-dots";

/** MapLibre's world is 512 px wide at zoom 0. */
const WORLD_PX = 512;
const DEGREES = 360;
/** Highest zoom the radius stop covers; past it the cells are far bigger than the screen anyway. */
const TOP_ZOOM = 24;
/**
 * A cell's blob radius, relative to its width: 0.9 for a lone detection, 1.4 for the busiest. The
 * apps use 0.5–1, but MapLibre's `circle-blur` fades faster than their linear gradient, and at 0.5
 * a region read as a lattice of dots instead of one patch of heat (checked on live FIRMS data).
 */
const CELL_RADIUS = { base: 0.9, span: 0.5 } as const;
const CELL_ALPHA = { base: 0.06, span: 0.74 } as const;
/** Dot radius, px: 1.5 for a weak or unknown reading up to 3 at 100 MW or more (3–6 px across). */
const DOT_RADIUS = { base: 1.5, span: 1.5 } as const;
/** log10 of the FRP that draws the biggest dot (100 MW). */
const FULL_FRP_LOG = 2;
const DOT_ALPHA = { low: 0.55, other: 0.95 } as const;
const OUTLINE = { width: 0.75, opacity: 0.8 } as const;

interface CellProperties {
  dot: false;
  /** Radius at zoom 0, px; the layer doubles it with each zoom so it keeps the cell's size on the map. */
  radius: number;
  alpha: number;
}

interface DotProperties {
  dot: true;
  radius: number;
  alpha: number;
}

type DetectionFeature = Feature<Point, CellProperties | DotProperties>;

function at(longitude: number, latitude: number): Point {
  return { type: "Point", coordinates: [longitude, latitude] };
}

/**
 * Cells scaled to the busiest in view on a log scale: a lone detection (a gas flare, a factory) is
 * a faint trace, a burning region the full tint. An absolute scale paints whole continents yellow.
 */
function cellFeatures(layer: DetectionLayer & { kind: "grid" }): DetectionFeature[] {
  let busiest = 1;
  for (const cell of layer.cells) {
    busiest = Math.max(busiest, cell.count);
  }
  const peak = Math.log1p(busiest);
  const cellWidth = (WORLD_PX * layer.cellDegrees) / DEGREES;
  return layer.cells.map((cell) => {
    const share = Math.log1p(cell.count) / peak;
    return {
      type: "Feature",
      geometry: at(cell.longitude, cell.latitude),
      properties: {
        dot: false,
        radius: cellWidth * (CELL_RADIUS.base + CELL_RADIUS.span * share),
        alpha: CELL_ALPHA.base + CELL_ALPHA.span * share * share,
      },
    };
  });
}

function dotFeatures(layer: DetectionLayer & { kind: "points" }): DetectionFeature[] {
  return layer.points.map((detection) => {
    const strength = Math.min(1, Math.log10(Math.max(detection.frp ?? 1, 1)) / FULL_FRP_LOG);
    return {
      type: "Feature",
      geometry: at(detection.longitude, detection.latitude),
      properties: {
        dot: true,
        radius: DOT_RADIUS.base + DOT_RADIUS.span * strength,
        alpha: detection.confidence === "low" ? DOT_ALPHA.low : DOT_ALPHA.other,
      },
    };
  });
}

function detectionFeatures(layer: DetectionLayer): FeatureCollection<Point, CellProperties | DotProperties> {
  return {
    type: "FeatureCollection",
    features: layer.kind === "grid" ? cellFeatures(layer) : dotFeatures(layer),
  };
}

const IS_DOT: ExpressionSpecification = ["==", ["get", "dot"], true];
const IS_CELL: ExpressionSpecification = ["==", ["get", "dot"], false];

/** Soft blobs the size of their cell on the map, fading from the centre. */
function cellLayer(): CircleLayerSpecification {
  return {
    id: DETECTION_CELLS,
    type: "circle",
    source: DETECTIONS_SOURCE,
    filter: IS_CELL,
    paint: {
      // Doubling per zoom keeps a radius fixed on the map: `radius` at zoom 0, times 2^zoom.
      "circle-radius": [
        "interpolate",
        ["exponential", 2],
        ["zoom"],
        0,
        ["get", "radius"],
        TOP_ZOOM,
        ["*", ["get", "radius"], 2 ** TOP_ZOOM],
      ],
      "circle-color": kindColors.detection,
      "circle-opacity": ["get", "alpha"],
      "circle-blur": 1,
      "circle-pitch-alignment": "map",
    },
  };
}

/** Dots that keep their screen size at every zoom, with a thin canvas outline. */
function dotLayer(): CircleLayerSpecification {
  return {
    id: DETECTION_DOTS,
    type: "circle",
    source: DETECTIONS_SOURCE,
    filter: IS_DOT,
    paint: {
      "circle-radius": ["get", "radius"],
      "circle-color": kindColors.detection,
      "circle-opacity": ["get", "alpha"],
      "circle-stroke-color": colorTokens.canvas,
      "circle-stroke-opacity": OUTLINE.opacity,
      "circle-stroke-width": OUTLINE.width,
    },
  };
}

function detectionLayers(): CircleLayerSpecification[] {
  return [cellLayer(), dotLayer()];
}

export { DETECTION_CELLS, DETECTION_DOTS, DETECTIONS_SOURCE, detectionFeatures, detectionLayers };

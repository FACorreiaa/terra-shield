import type { HazardEvent } from "@shield/api-client";
import type { Feature, FeatureCollection, LineString, Polygon } from "geojson";
import type { ExpressionSpecification, FillLayerSpecification, LineLayerSpecification } from "maplibre-gl";

import { alphaTokens, severityColors } from "@/design/tokens.gen.ts";
import type { Coordinate, EventShape } from "@/domain/geo.ts";

/**
 * The selected event's footprint (DESIGN.md §5, §9), drawn under every marker. Kind sets the
 * form: observed 18% fill + 1.5 px line, official warning 10% fill + 2 px line, forecast no fill
 * and a dashed 1.5 px line. Colour stays with severity. Stale or resolved areas fade.
 */

const AREA_SOURCE = "area";
const AREA_FILL = "area-fill";
const AREA_LINE = "area-line";
const AREA_LINE_FORECAST = "area-line-forecast";

const LINE = { observed: 1.5, warning: 2, forecast: 1.5 } as const;
const FORECAST_DASH = { on: 6, off: 4 } as const;

interface AreaProperties {
  color: string;
  fill: number;
  line: number;
  opacity: number;
  forecast: boolean;
}

function fillAlpha(kind: HazardEvent["kind"]): number {
  if (kind === "observed") {
    return alphaTokens.areaFillObserved;
  }
  return kind === "warning" ? alphaTokens.areaFillWarning : 0;
}

function lineWidth(kind: HazardEvent["kind"]): number {
  return kind === "warning" ? LINE.warning : LINE.observed;
}

function toPosition({ longitude, latitude }: Coordinate): [number, number] {
  return [longitude, latitude];
}

function geometries(shape: EventShape): (Polygon | LineString)[] {
  if (shape.kind === "polygons") {
    return shape.polygons.map((rings) => ({
      type: "Polygon",
      coordinates: rings.map((ring) => ring.map((point) => toPosition(point))),
    }));
  }
  if (shape.kind === "lines") {
    return shape.lines.map((line) => ({ type: "LineString", coordinates: line.map((point) => toPosition(point)) }));
  }
  return [];
}

/** The footprint as features; empty for a point (the marker already is the point) or no selection. */
function areaFeatures(
  selected: { event: HazardEvent; shape: EventShape; faded: boolean } | undefined,
): FeatureCollection<Polygon | LineString, AreaProperties> {
  if (selected === undefined) {
    return { type: "FeatureCollection", features: [] };
  }
  const { event, shape, faded } = selected;
  const properties: AreaProperties = {
    color: severityColors[event.severity],
    fill: fillAlpha(event.kind),
    line: lineWidth(event.kind),
    opacity: faded ? alphaTokens.stale : 1,
    forecast: event.kind === "forecast",
  };
  return {
    type: "FeatureCollection",
    features: geometries(shape).map((geometry): Feature<Polygon | LineString, AreaProperties> => ({
      type: "Feature",
      geometry,
      properties,
    })),
  };
}

function areaLayers(): (FillLayerSpecification | LineLayerSpecification)[] {
  const isForecast: ExpressionSpecification = ["==", ["get", "forecast"], true];
  return [
    {
      id: AREA_FILL,
      type: "fill",
      source: AREA_SOURCE,
      filter: ["==", ["geometry-type"], "Polygon"],
      paint: {
        "fill-color": ["get", "color"],
        "fill-opacity": ["*", ["get", "fill"], ["get", "opacity"]],
      },
    },
    {
      id: AREA_LINE,
      type: "line",
      source: AREA_SOURCE,
      filter: ["!", isForecast],
      paint: {
        "line-color": ["get", "color"],
        "line-width": ["get", "line"],
        "line-opacity": ["get", "opacity"],
      },
    },
    {
      id: AREA_LINE_FORECAST,
      type: "line",
      source: AREA_SOURCE,
      filter: isForecast,
      paint: {
        "line-color": ["get", "color"],
        "line-width": LINE.forecast,
        "line-opacity": ["get", "opacity"],
        "line-dasharray": [FORECAST_DASH.on, FORECAST_DASH.off],
      },
    },
  ];
}

export type { AreaProperties };
export { AREA_SOURCE, areaFeatures, areaLayers };

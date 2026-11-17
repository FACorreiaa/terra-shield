import type { HazardEvent } from "@shield/api-client";
import type { FeatureCollection, Point } from "geojson";
import type { SymbolLayerSpecification } from "maplibre-gl";

import { severityTokens } from "@/design/tokens.gen.ts";

/** Marker images are drawn at 2× (markerImages.ts); half of that is the base size. */
const BASE_ICON_SIZE = 0.5;

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/** Detections (FIRMS) get their own density layer later (DESIGN.md §5), never severity shapes. */
function eventFeatures(events: readonly HazardEvent[]): FeatureCollection<Point> {
  return {
    type: "FeatureCollection",
    features: events
      .filter((event) => event.kind !== "detection")
      .map((event) => ({
        type: "Feature",
        id: event.id,
        geometry: { type: "Point", coordinates: [event.centroid.longitude, event.centroid.latitude] },
        properties: {
          id: event.id,
          severity: event.severity,
          shape: severityTokens[event.severity].shape,
          scale: severityTokens[event.severity].markerScale,
          outline: event.kind === "forecast",
        },
      })),
  };
}

function eventLayer(): SymbolLayerSpecification {
  return {
    id: "events",
    type: "symbol",
    source: "events",
    layout: {
      "icon-image": [
        "case",
        ["get", "outline"],
        ["concat", "shape-", ["get", "shape"], "-outline"],
        ["concat", "shape-", ["get", "shape"]],
      ],
      "icon-size": ["*", BASE_ICON_SIZE, ["get", "scale"]],
      "icon-allow-overlap": true,
      "symbol-sort-key": ["get", "scale"],
    },
    paint: {
      "icon-color": [
        "match",
        ["get", "severity"],
        "extreme",
        cssVar("--color-severity-extreme"),
        "severe",
        cssVar("--color-severity-severe"),
        "moderate",
        cssVar("--color-severity-moderate"),
        cssVar("--color-severity-minor"),
      ],
      "icon-halo-color": cssVar("--color-canvas"),
      "icon-halo-width": 1,
    },
  };
}

export { cssVar, eventFeatures, eventLayer };

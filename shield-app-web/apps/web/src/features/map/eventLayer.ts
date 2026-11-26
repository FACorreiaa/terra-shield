import type { HazardEvent } from "@shield/api-client";
import { hazardType } from "@shield/api-client";
import type { FeatureCollection, Point } from "geojson";
import type { CircleLayerSpecification, SymbolLayerSpecification } from "maplibre-gl";

import { colorTokens } from "@/design/tokens.gen.ts";
import { severityRank } from "@/domain/taxonomy.ts";

import type { MarkerKind, MarkerState } from "./markerArt.ts";
import { markerImageName, markerSize } from "./markerArt.ts";
import type { MarkerRequest } from "./markerImages.ts";

const EVENTS_SOURCE = "events";
const EVENTS_LAYER = "events";
const SELECTION_LAYER = "events-selection";

/** Active markers draw over resolved ones; within each, the more severe on top. */
const ACTIVE_PRIORITY = 10;
/** The selection ring sits this far outside the shape, in px (HazardMarker.swift: padding -3). */
const SELECTION_GAP = 3;
const SELECTION_WIDTH = 2;

interface EventProperties {
  id: string;
  image: string;
  priority: number;
  /** Radius of the selection ring for this marker's size. */
  ring: number;
}

/** Points the map draws as hazard markers: no heat detections (their own layer) and no conflicts. */
function isMarker(event: HazardEvent): event is HazardEvent & { kind: MarkerKind } {
  return event.kind !== "detection" && event.type !== "conflict";
}

function markerState(event: HazardEvent, staleSources: ReadonlySet<string>): MarkerState {
  if (event.status === "resolved") {
    return "resolved";
  }
  return staleSources.has(event.source) ? "stale" : "live";
}

function request(event: HazardEvent & { kind: MarkerKind }, staleSources: ReadonlySet<string>): MarkerRequest {
  return {
    type: hazardType(event.type),
    severity: event.severity,
    kind: event.kind,
    state: markerState(event, staleSources),
  };
}

/** The marker images a set of events needs, each once. */
function markerRequests(events: readonly HazardEvent[], staleSources: ReadonlySet<string>): MarkerRequest[] {
  const unique = new Map<string, MarkerRequest>();
  for (const event of events.filter((candidate) => isMarker(candidate))) {
    const needed = request(event, staleSources);
    unique.set(markerImageName(needed.type, needed), needed);
  }
  return [...unique.values()];
}

function eventFeatures(
  events: readonly HazardEvent[],
  staleSources: ReadonlySet<string>,
): FeatureCollection<Point, EventProperties> {
  return {
    type: "FeatureCollection",
    features: events
      .filter((event) => isMarker(event))
      .map((event) => {
        const needed = request(event, staleSources);
        return {
          type: "Feature",
          id: event.id,
          geometry: { type: "Point", coordinates: [event.centroid.longitude, event.centroid.latitude] },
          properties: {
            id: event.id,
            image: markerImageName(needed.type, needed),
            priority: severityRank(event.severity) + (event.status === "resolved" ? 0 : ACTIVE_PRIORITY),
            ring: markerSize(event.severity) / 2 + SELECTION_GAP,
          },
        };
      }),
  };
}

function eventLayer(): SymbolLayerSpecification {
  return {
    id: EVENTS_LAYER,
    type: "symbol",
    source: EVENTS_SOURCE,
    layout: {
      "icon-image": ["get", "image"],
      "icon-allow-overlap": true,
      "icon-ignore-placement": true,
      "symbol-sort-key": ["get", "priority"],
    },
  };
}

/** An `ink` ring around the selected marker, drawn under it. */
function selectionLayer(selectedId: string | undefined): CircleLayerSpecification {
  return {
    id: SELECTION_LAYER,
    type: "circle",
    source: EVENTS_SOURCE,
    filter: ["==", ["get", "id"], selectedId ?? ""],
    paint: {
      "circle-radius": ["get", "ring"],
      "circle-color": colorTokens.canvas,
      "circle-opacity": 0,
      "circle-stroke-color": colorTokens.ink,
      "circle-stroke-width": SELECTION_WIDTH,
    },
  };
}

export type { EventProperties };
export {
  EVENTS_LAYER,
  EVENTS_SOURCE,
  eventFeatures,
  eventLayer,
  markerRequests,
  SELECTION_GAP,
  SELECTION_LAYER,
  selectionLayer,
};

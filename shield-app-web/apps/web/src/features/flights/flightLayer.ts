import type { Aircraft } from "@shield/api-client";
import type { FeatureCollection, Point } from "geojson";
import type { CircleLayerSpecification, SymbolLayerSpecification } from "maplibre-gl";

import { colorTokens } from "@/design/tokens.gen.ts";

/**
 * Live flights on the canvas (ADR 28, `Flights.swift`): their own source and layers, under every
 * hazard marker. Hazards are the app's subject, flights context, so aircraft never join the
 * hazards' source, sort keys or clusters, and give way to each other where they crowd.
 */
const AIRCRAFT_SOURCE = "aircraft";
const AIRCRAFT_LAYER = "aircraft";
const AIRCRAFT_SELECTION_LAYER = "aircraft-selection";

const PLANE_IMAGE = "aircraft/plane";
const DOT_IMAGE = "aircraft/dot";

/** Drawn at twice their size so they stay sharp on high-density screens. */
const PIXEL_RATIO = 2;
/** The marker's box, as the apps' 28 pt annotation. */
const BOX = 28;
const DOT_RADIUS = 3.5;
/** The apps' 25 % `accent` disc behind the selected aircraft. */
const SELECTION_RADIUS = 14;
const SELECTION_OPACITY = 0.25;
/** A thin `canvas` edge stands in for the apps' shadow, so the glyph reads over any basemap. */
const EDGE = 1.5;
const VIEWBOX = 24;

interface AircraftProperties {
  hex: string;
  image: string;
  /** Degrees clockwise from north; 0 for a dot, which has no direction. */
  rotate: number;
}

const NO_FEATURES: FeatureCollection<Point, AircraftProperties> = { type: "FeatureCollection", features: [] };

/** One point per aircraft: the plane turned to its track, or a dot without one (direction unknown, not guessed). */
function aircraftFeatures(aircraft: readonly Aircraft[] | undefined): FeatureCollection<Point, AircraftProperties> {
  if (aircraft === undefined || aircraft.length === 0) {
    return NO_FEATURES;
  }
  return {
    type: "FeatureCollection",
    features: aircraft.map((each) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [each.lon, each.lat] },
      properties: {
        hex: each.hex,
        image: each.trackDeg === undefined ? DOT_IMAGE : PLANE_IMAGE,
        rotate: each.trackDeg ?? 0,
      },
    })),
  };
}

/** The plane points north at rest, so the track turns it as is; turned with the map, not the screen. */
function aircraftLayer(): SymbolLayerSpecification {
  return {
    id: AIRCRAFT_LAYER,
    type: "symbol",
    source: AIRCRAFT_SOURCE,
    layout: {
      "icon-image": ["get", "image"],
      "icon-rotate": ["get", "rotate"],
      "icon-rotation-alignment": "map",
      // Aircraft hide one another where they crowd, as MapKit's collision does.
      "icon-allow-overlap": false,
      "icon-padding": 0,
    },
  };
}

function aircraftSelectionLayer(hex: string | undefined): CircleLayerSpecification {
  return {
    id: AIRCRAFT_SELECTION_LAYER,
    type: "circle",
    source: AIRCRAFT_SOURCE,
    filter: ["==", ["get", "hex"], hex ?? ""],
    paint: {
      "circle-radius": SELECTION_RADIUS,
      "circle-color": colorTokens.accent,
      "circle-opacity": SELECTION_OPACITY,
    },
  };
}

/** An airliner seen from above, nose up, in a 24-unit box. */
const PLANE_PATH =
  "M12 2c.8 0 1.4.7 1.4 1.5V9l7.6 4.4v2l-7.6-2.3v4.6l2.2 1.7V21L12 20l-3.6 1v-1.6l2.2-1.7v-4.6L3 15.4v-2L10.6 9V3.5C10.6 2.7 11.2 2 12 2z";

function svg(body: string): string {
  const size = BOX * PIXEL_RATIO;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${BOX} ${BOX}">${body}</svg>`;
}

/** The two pictures the layer draws, as SVG markup: `ink` with a `canvas` edge. */
function aircraftImages(): Record<string, string> {
  const inset = (BOX - VIEWBOX) / 2;
  const stroke = `stroke="${colorTokens.canvas}" stroke-width="${EDGE}" stroke-linejoin="round"`;
  return {
    [PLANE_IMAGE]: svg(
      `<path transform="translate(${inset} ${inset})" d="${PLANE_PATH}" fill="${colorTokens.ink}" ${stroke}/>`,
    ),
    [DOT_IMAGE]: svg(`<circle cx="${BOX / 2}" cy="${BOX / 2}" r="${DOT_RADIUS}" fill="${colorTokens.ink}" ${stroke}/>`),
  };
}

export type { AircraftProperties };
export {
  AIRCRAFT_LAYER,
  AIRCRAFT_SELECTION_LAYER,
  AIRCRAFT_SOURCE,
  aircraftFeatures,
  aircraftImages,
  aircraftLayer,
  aircraftSelectionLayer,
  DOT_IMAGE,
  PIXEL_RATIO,
  PLANE_IMAGE,
};

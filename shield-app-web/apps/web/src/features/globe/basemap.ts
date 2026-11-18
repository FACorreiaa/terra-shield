import type { FeatureCollection, Geometry, Position } from "geojson";
import type { StyleSpecification } from "maplibre-gl";
import { feature } from "topojson-client";
import type { Topology } from "topojson-specification";
import countries110m from "world-atlas/countries-110m.json" with { type: "json" };

function isTopology(value: unknown): value is Topology {
  return typeof value === "object" && value !== null && "type" in value && value.type === "Topology";
}

const HALF_TURN = 180;
const FULL_TURN = 360;

/**
 * Rings that cross the antimeridian jump from 180 to -180 between neighbouring points, which a
 * flat map draws as a streak across the world. Shifting by ±360 keeps every edge short.
 */
function unwrapRing(ring: readonly Position[]): Position[] {
  let offset = 0;
  let previous = Number.NaN;
  return ring.map(([lon = 0, lat = 0]) => {
    if (!Number.isNaN(previous)) {
      const jump = lon + offset - previous;
      if (jump > HALF_TURN) {
        offset -= FULL_TURN;
      } else if (jump < -HALF_TURN) {
        offset += FULL_TURN;
      }
    }
    previous = lon + offset;
    return [previous, lat];
  });
}

function unwrapGeometry(geometry: Geometry): Geometry {
  if (geometry.type === "Polygon") {
    return { ...geometry, coordinates: geometry.coordinates.map((ring) => unwrapRing(ring)) };
  }
  if (geometry.type === "MultiPolygon") {
    return {
      ...geometry,
      coordinates: geometry.coordinates.map((polygon) => polygon.map((ring) => unwrapRing(ring))),
    };
  }
  return geometry;
}

/**
 * Near-black navy basemap from Natural Earth (public domain) via world-atlas, bundled, so no
 * third-party tiles or licence are involved. A detailed tile basemap is an open owner verdict
 * (DESIGN.md §9: Protomaps / OpenFreeMap after a licence check).
 */
function countriesGeoJSON(): FeatureCollection {
  const topology: unknown = countries110m;
  if (!isTopology(topology)) {
    throw new Error("world-atlas: not a TopoJSON topology");
  }
  const { countries } = topology.objects;
  if (countries === undefined) {
    throw new Error("world-atlas: countries object missing");
  }
  const collection = feature(topology, countries);
  if (collection.type !== "FeatureCollection") {
    throw new Error("world-atlas: countries is not a collection");
  }
  for (const country of collection.features) {
    country.geometry = unwrapGeometry(country.geometry);
  }
  return collection;
}

interface BasemapColors {
  canvas: string;
  panel: string;
  hairline: string;
}

const BORDER_WIDTH = 0.6;

function basemapStyle(colors: BasemapColors): StyleSpecification {
  return {
    version: 8,
    sources: {
      countries: { type: "geojson", data: countriesGeoJSON() },
    },
    layers: [
      { id: "background", type: "background", paint: { "background-color": colors.canvas } },
      { id: "countries-fill", type: "fill", source: "countries", paint: { "fill-color": colors.panel } },
      {
        id: "countries-line",
        type: "line",
        source: "countries",
        paint: { "line-color": colors.hairline, "line-width": BORDER_WIDTH },
      },
    ],
  };
}

export { basemapStyle };

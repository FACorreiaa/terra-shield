import type { ExpressionSpecification, LayerSpecification, SkySpecification, StyleSpecification } from "maplibre-gl";

import { colorTokens } from "@/design/tokens.gen.ts";
import { mix } from "@/lib/color.ts";

import type { BasemapPalette } from "./basemapPalette.ts";
import { BLEND, palette } from "./basemapPalette.ts";

/**
 * The Aegis Terra basemap (DESIGN.md §9): OpenFreeMap's OpenMapTiles vector tiles under a quiet
 * navy style built from tokens, so the hazard layer is the loudest thing on screen. Water is a step
 * darker than land, admin-0 borders are `hairline`, labels `faint`, no roads below z8, no POIs.
 * OpenFreeMap allows commercial use without a key; a customised style must be self-hosted, which
 * this is (bundled). Attribution comes from the source's TileJSON.
 */

const OPENFREEMAP_TILES = "https://tiles.openfreemap.org/planet";
/**
 * OpenFreeMap's required credit (DESIGN.md §9), declared here so it never depends on their
 * TileJSON carrying it.
 */
const OPENFREEMAP_ATTRIBUTION = [
  '<a href="https://openfreemap.org" target="_blank" rel="noopener">OpenFreeMap</a>',
  '<a href="https://www.openmaptiles.org/" target="_blank" rel="noopener">© OpenMapTiles</a>',
  'Data from <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a>',
].join(" ");
const OPENFREEMAP_GLYPHS = "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf";
const SOURCE = "openmaptiles";

/** Linear interpolation over zoom, written as `{ zoom: value }`. */
function ramp(stops: Record<number, number>): ExpressionSpecification {
  const flat = Object.entries(stops)
    .map(([zoom, value]) => [Number(zoom), value] as const)
    .toSorted(([left], [right]) => left - right)
    .flat();
  return ["interpolate", ["linear"], ["zoom"], ...flat];
}

/** Minimum zooms: quiet at world scale, detail only once regional. */
const MIN_ZOOM = {
  minorRoads: 10,
  majorRoads: 8,
  admin1: 4,
  waterLabels: 3,
  cityLabels: 4,
  stateLabels: 5,
  townLabels: 8,
  countryLabels: 1,
  buildings: 14,
} as const;

const SIZE = {
  townLabel: ramp({ 8: 10, 14: 13 }),
  cityLabel: ramp({ 4: 10, 10: 15 }),
  stateLabel: ramp({ 5: 10, 8: 12 }),
  countryLabel: ramp({ 1: 10, 3: 12, 7: 16 }),
  waterLabel: ramp({ 3: 10, 8: 13 }),
  minorRoad: ramp({ 10: 0.4, 16: 3 }),
  majorRoad: ramp({ 8: 0.5, 16: 5 }),
  admin1: ramp({ 4: 0.4, 10: 1 }),
  admin0: ramp({ 1: 0.6, 6: 1.2, 10: 2 }),
  disputed: ramp({ 1: 0.6, 10: 1.6 }),
} as const;

const DASH = { admin1: { on: 3, off: 2 }, disputed: { on: 2, off: 2 } } as const;
const ADMIN_LEVEL = { country: 2, region: 4 } as const;
const TEXT = {
  maxWidth: 7,
  waterMaxWidth: 6,
  waterSpacing: 0.1,
  halo: 1.2,
  haloBlur: 0.5,
  buildingOpacity: 0.5,
} as const;

const FONT = { regular: ["Noto Sans Regular"], bold: ["Noto Sans Bold"], italic: ["Noto Sans Italic"] } as const;
const NAME: ExpressionSpecification = ["coalesce", ["get", "name:en"], ["get", "name_en"], ["get", "name"]];

function dash(pattern: { on: number; off: number }): number[] {
  return [pattern.on, pattern.off];
}

function surfaceLayers(colors: BasemapPalette): LayerSpecification[] {
  return [
    { id: "land", type: "background", paint: { "background-color": colors.land } },
    {
      id: "ice",
      type: "fill",
      source: SOURCE,
      "source-layer": "landcover",
      filter: ["in", ["get", "subclass"], ["literal", ["glacier", "ice_shelf"]]],
      paint: { "fill-color": colors.ice },
    },
    {
      id: "water",
      type: "fill",
      source: SOURCE,
      "source-layer": "water",
      paint: { "fill-color": colors.ocean, "fill-antialias": false },
    },
    {
      id: "building",
      type: "fill",
      source: SOURCE,
      "source-layer": "building",
      minzoom: MIN_ZOOM.buildings,
      paint: { "fill-color": colors.road, "fill-opacity": TEXT.buildingOpacity },
    },
  ];
}

function lineLayers(colors: BasemapPalette): LayerSpecification[] {
  const notMaritime: ExpressionSpecification = ["!=", ["get", "maritime"], 1];
  return [
    {
      id: "road-minor",
      type: "line",
      source: SOURCE,
      "source-layer": "transportation",
      minzoom: MIN_ZOOM.minorRoads,
      filter: ["in", ["get", "class"], ["literal", ["minor", "service", "tertiary"]]],
      paint: { "line-color": colors.road, "line-width": SIZE.minorRoad },
    },
    {
      id: "road-major",
      type: "line",
      source: SOURCE,
      "source-layer": "transportation",
      minzoom: MIN_ZOOM.majorRoads,
      filter: ["in", ["get", "class"], ["literal", ["motorway", "trunk", "primary", "secondary"]]],
      paint: { "line-color": colors.roadMajor, "line-width": SIZE.majorRoad },
    },
    {
      id: "admin-1",
      type: "line",
      source: SOURCE,
      "source-layer": "boundary",
      minzoom: MIN_ZOOM.admin1,
      filter: ["all", ["==", ["get", "admin_level"], ADMIN_LEVEL.region], notMaritime],
      paint: { "line-color": colors.admin1, "line-width": SIZE.admin1, "line-dasharray": dash(DASH.admin1) },
    },
    {
      id: "admin-0",
      type: "line",
      source: SOURCE,
      "source-layer": "boundary",
      filter: ["all", ["==", ["get", "admin_level"], ADMIN_LEVEL.country], notMaritime, ["!=", ["get", "disputed"], 1]],
      paint: { "line-color": colors.admin0, "line-width": SIZE.admin0 },
    },
    {
      id: "admin-0-disputed",
      type: "line",
      source: SOURCE,
      "source-layer": "boundary",
      filter: ["all", ["==", ["get", "admin_level"], ADMIN_LEVEL.country], ["==", ["get", "disputed"], 1]],
      paint: { "line-color": colors.admin0, "line-width": SIZE.disputed, "line-dasharray": dash(DASH.disputed) },
    },
  ];
}

interface PlaceLabel {
  id: string;
  minzoom: number;
  classes: readonly string[];
  size: ExpressionSpecification;
  color: string;
  font?: readonly string[];
}

function placeLabel(label: PlaceLabel, halo: string): LayerSpecification {
  return {
    id: label.id,
    type: "symbol",
    source: SOURCE,
    "source-layer": "place",
    minzoom: label.minzoom,
    filter: ["in", ["get", "class"], ["literal", label.classes]],
    layout: {
      "text-field": NAME,
      "text-font": [...(label.font ?? FONT.regular)],
      "text-size": label.size,
      "text-max-width": TEXT.maxWidth,
    },
    paint: {
      "text-color": label.color,
      "text-halo-color": halo,
      "text-halo-width": TEXT.halo,
      "text-halo-blur": TEXT.haloBlur,
    },
  };
}

function labelLayers(colors: BasemapPalette): LayerSpecification[] {
  const places: PlaceLabel[] = [
    {
      id: "town-label",
      minzoom: MIN_ZOOM.townLabels,
      classes: ["town", "village"],
      size: SIZE.townLabel,
      color: colors.labelMinor,
    },
    {
      id: "city-label",
      minzoom: MIN_ZOOM.cityLabels,
      classes: ["city"],
      size: SIZE.cityLabel,
      color: colors.labelMinor,
    },
    {
      id: "state-label",
      minzoom: MIN_ZOOM.stateLabels,
      classes: ["state"],
      size: SIZE.stateLabel,
      color: colors.labelMinor,
    },
    {
      id: "country-label",
      minzoom: MIN_ZOOM.countryLabels,
      classes: ["country"],
      size: SIZE.countryLabel,
      color: colors.label,
      font: FONT.bold,
    },
  ];
  return [
    {
      id: "water-label",
      type: "symbol",
      source: SOURCE,
      "source-layer": "water_name",
      minzoom: MIN_ZOOM.waterLabels,
      filter: ["==", ["geometry-type"], "Point"],
      layout: {
        "text-field": NAME,
        "text-font": [...FONT.italic],
        "text-size": SIZE.waterLabel,
        "text-max-width": TEXT.waterMaxWidth,
        "text-letter-spacing": TEXT.waterSpacing,
      },
      paint: { "text-color": colors.waterLabel, "text-halo-color": colors.halo, "text-halo-width": 1 },
    },
    ...places.map((label) => placeLabel(label, colors.halo)),
  ];
}

/** The globe's atmospheric edge (DESIGN.md §8 allows it): a faint accent rim that fades as you zoom in. */
function sky(): SkySpecification {
  const { canvas, accent } = colorTokens;
  return {
    "sky-color": canvas,
    "horizon-color": mix(canvas, accent, BLEND.atmosphereTowardAccent),
    "fog-color": canvas,
    "atmosphere-blend": ramp({ 0: 0.35, 3: 0.2, 6: 0 }),
  };
}

type Projection = "globe" | "mercator";

/** The complete style. Hazard layers are added on top by the map, never here. */
function basemapStyle(projection: Projection): StyleSpecification {
  const colors = palette();
  return {
    version: 8,
    projection: { type: projection },
    sky: sky(),
    glyphs: OPENFREEMAP_GLYPHS,
    sources: { [SOURCE]: { type: "vector", url: OPENFREEMAP_TILES, attribution: OPENFREEMAP_ATTRIBUTION } },
    layers: [...surfaceLayers(colors), ...lineLayers(colors), ...labelLayers(colors)],
  };
}

export type { Projection };
export { basemapStyle, OPENFREEMAP_TILES };

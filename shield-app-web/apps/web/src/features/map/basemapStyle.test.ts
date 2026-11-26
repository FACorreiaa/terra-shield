import type { LayerSpecification } from "maplibre-gl";
import { describe, expect, it } from "vite-plus/test";

import { colorTokens } from "@/design/tokens.gen.ts";

import { basemapStyle } from "./basemapStyle.ts";

const ROADS_MIN_ZOOM = 8;
const style = basemapStyle("globe");

function layer(id: string): LayerSpecification | undefined {
  return style.layers.find((candidate) => candidate.id === id);
}

/** A paint property of a layer, whatever its type. */
function paint(id: string, property: string): unknown {
  const found = layer(id);
  return found !== undefined && "paint" in found && found.paint !== undefined
    ? (found.paint as Record<string, unknown>)[property]
    : undefined;
}

function sourceLayer(candidate: LayerSpecification): string | undefined {
  return "source-layer" in candidate ? candidate["source-layer"] : undefined;
}

const roads = style.layers.filter((candidate) => sourceLayer(candidate) === "transportation");
const roadMinZooms = roads.map((road) => road.minzoom ?? 0);

describe(basemapStyle, () => {
  it("draws water in canvas, a step darker than land", () => {
    expect(paint("water", "fill-color")).toBe(colorTokens.canvas);
    expect(paint("land", "background-color")).not.toBe(colorTokens.canvas);
  });

  it("shows no roads below zoom 8 and no points of interest", () => {
    expect(roads.length).toBeGreaterThan(0);
    expect(Math.min(...roadMinZooms)).toBeGreaterThanOrEqual(ROADS_MIN_ZOOM);
    expect(style.layers.map((candidate) => sourceLayer(candidate))).not.toContain("poi");
  });

  it("draws country borders in hairline", () => {
    expect(paint("admin-0", "line-color")).toBe(colorTokens.hairline);
  });

  it("carries the requested projection", () => {
    expect(basemapStyle("mercator").projection).toStrictEqual({ type: "mercator" });
    expect(style.projection).toStrictEqual({ type: "globe" });
  });
});

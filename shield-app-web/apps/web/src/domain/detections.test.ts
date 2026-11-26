import type { components } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import type { DetectionLayer } from "./detections.ts";
import { detectionLayer, detectionLegend, EMPTY_DETECTIONS, sameDetections } from "./detections.ts";

type FireCollection = components["schemas"]["FireFeatureCollection"];

const NOW = new Date("2026-11-26T12:00:00Z");

type FireFeature = FireCollection["features"][number];

function point(coordinates: unknown, properties: FireFeature["properties"]): FireFeature {
  return { type: "Feature", geometry: { type: "Point", coordinates }, properties };
}

const GRID: FireCollection = {
  type: "FeatureCollection",
  aggregation: "grid",
  cellDegrees: 2,
  truncated: false,
  features: [
    point([10, 5], { count: 40, maxFrp: 120, latestAt: "2026-11-26T09:00:00Z" }),
    point([12, 5], { count: 1, latestAt: "2026-11-26T08:00:00Z" }),
  ],
};

const POINTS: FireCollection = {
  type: "FeatureCollection",
  aggregation: "none",
  truncated: true,
  features: [
    point([-60.5, -10.2], {
      satellite: "NOAA-20",
      acquiredAt: "2026-11-26T10:30:00Z",
      frp: 42.5,
      confidence: "high",
    }),
    point([-60.6, -10.3], { satellite: "Suomi NPP", acquiredAt: "2026-11-26T07:00:00Z" }),
  ],
};

describe(detectionLayer, () => {
  it("reads a grid answer as cells", () => {
    expect(detectionLayer(GRID)).toStrictEqual({
      kind: "grid",
      cellDegrees: 2,
      latest: new Date("2026-11-26T09:00:00Z"),
      cells: [
        { longitude: 10, latitude: 5, count: 40, maxFrp: 120, latestAt: new Date("2026-11-26T09:00:00Z") },
        { longitude: 12, latitude: 5, count: 1, maxFrp: undefined, latestAt: new Date("2026-11-26T08:00:00Z") },
      ],
    });
  });

  it("reads a point answer as detections, keeping truncated", () => {
    expect(detectionLayer(POINTS)).toStrictEqual({
      kind: "points",
      truncated: true,
      latest: new Date("2026-11-26T10:30:00Z"),
      points: [
        {
          longitude: -60.5,
          latitude: -10.2,
          satellite: "NOAA-20",
          acquiredAt: new Date("2026-11-26T10:30:00Z"),
          frp: 42.5,
          confidence: "high",
        },
        {
          longitude: -60.6,
          latitude: -10.3,
          satellite: "Suomi NPP",
          acquiredAt: new Date("2026-11-26T07:00:00Z"),
          frp: undefined,
          confidence: undefined,
        },
      ],
    });
  });

  it("goes by aggregation, and skips features that aren't usable points", () => {
    const layer = detectionLayer({
      ...GRID,
      features: [
        // A detection's properties in a grid answer, a bad point, a bad date, a polygon: all skipped.
        point([1, 2], { satellite: "NOAA-20", acquiredAt: "2026-11-26T10:30:00Z" }),
        point(["a", 2], { count: 3, latestAt: "2026-11-26T09:00:00Z" }),
        point([1, 2], { count: 3, latestAt: "not a date" }),
        { type: "Feature", geometry: { type: "Polygon", coordinates: [] }, properties: { count: 1, latestAt: "" } },
        point([3, 4], { count: 3, latestAt: "2026-11-26T09:00:00Z" }),
      ],
    });
    expect(layer).toStrictEqual({
      kind: "grid",
      cellDegrees: 2,
      latest: new Date("2026-11-26T09:00:00Z"),
      cells: [{ longitude: 3, latitude: 4, count: 3, maxFrp: undefined, latestAt: new Date("2026-11-26T09:00:00Z") }],
    });
  });

  it("has no newest acquisition when empty", () => {
    expect(EMPTY_DETECTIONS.latest).toBeUndefined();
    expect(detectionLayer({ ...POINTS, features: [] }).latest).toBeUndefined();
  });
});

describe(sameDetections, () => {
  it("matches a reload that draws the same, field by field", () => {
    expect(sameDetections(detectionLayer(GRID), detectionLayer(GRID))).toBe(true);
    expect(sameDetections(detectionLayer(POINTS), detectionLayer(POINTS))).toBe(true);
  });

  it("tells apart a changed count, a new point, a cut-off answer and another kind", () => {
    const busier = structuredClone(GRID);
    busier.features[0] = point([20, 0], { count: 41, maxFrp: 120, latestAt: "2026-11-26T09:00:00Z" });
    expect(sameDetections(detectionLayer(GRID), detectionLayer(busier))).toBe(false);
    const more = { ...POINTS, features: [...POINTS.features, ...POINTS.features] };
    expect(sameDetections(detectionLayer(POINTS), detectionLayer(more))).toBe(false);
    expect(sameDetections(detectionLayer(POINTS), detectionLayer({ ...POINTS, truncated: false }))).toBe(false);
    expect(sameDetections(detectionLayer(GRID), detectionLayer(POINTS))).toBe(false);
  });
});

describe(detectionLegend, () => {
  const idle = { failed: false, loading: false, now: NOW };

  it("says how long ago the newest was detected, never 'updated'", () => {
    expect(detectionLegend(detectionLayer(GRID), idle)).toBe("Satellite heat detections · Detected 3 h ago");
  });

  it("asks to zoom in when points were cut off", () => {
    expect(detectionLegend(detectionLayer(POINTS), idle)).toBe(
      "Satellite heat detections · Detected 1 h ago · Zoom in for all",
    );
  });

  it("says loading, then none, for an empty layer", () => {
    expect(detectionLegend(EMPTY_DETECTIONS, { ...idle, loading: true })).toBe("Satellite heat detections · Loading…");
    expect(detectionLegend(EMPTY_DETECTIONS, idle)).toBe("Satellite heat detections · None in this view");
  });

  it("says when the layer didn't load, over what it last showed", () => {
    const layer: DetectionLayer = detectionLayer(GRID);
    expect(detectionLegend(layer, { ...idle, failed: true })).toBe("Heat detections didn’t load");
  });
});

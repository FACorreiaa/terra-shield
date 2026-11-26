import { describe, expect, it } from "vite-plus/test";

import { colorTokens, kindColors } from "@/design/tokens.gen.ts";
import type { DetectionCell, DetectionLayer } from "@/domain/detections.ts";

import { DETECTION_CELLS, DETECTION_DOTS, detectionFeatures, detectionLayers } from "./detectionLayer.ts";

const AT = new Date("2026-11-26T09:00:00Z");

function cell(count: number): DetectionCell {
  return { longitude: 0, latitude: 0, count, maxFrp: undefined, latestAt: AT };
}

describe(detectionFeatures, () => {
  it("scales cells to the busiest in view on a log scale", () => {
    const layer: DetectionLayer = { kind: "grid", cellDegrees: 1, cells: [cell(99), cell(1)], latest: AT };
    const [busiest, lone] = detectionFeatures(layer).features.map((feature) => feature.properties);
    // A cell 1° wide is 512/360 px at zoom 0; the busiest's blob is 1.4 cells, alpha 0.06 + 0.74.
    expect(busiest).toStrictEqual({ dot: false, radius: (512 / 360) * 1.4, alpha: 0.8 });
    expect(lone?.radius).toBeCloseTo((512 / 360) * (0.9 + 0.5 * (Math.log1p(1) / Math.log1p(99))));
    expect(lone?.alpha).toBeLessThan(0.1);
  });

  it("sizes dots by FRP, 1.5 to 3 px radius, and fades low confidence", () => {
    const base = { longitude: 0, latitude: 0, satellite: "NOAA-20", acquiredAt: AT };
    const layer: DetectionLayer = {
      kind: "points",
      truncated: false,
      latest: AT,
      points: [
        { ...base, frp: undefined, confidence: "low" },
        { ...base, frp: 10, confidence: "nominal" },
        { ...base, frp: 500, confidence: "high" },
      ],
    };
    expect(detectionFeatures(layer).features.map((feature) => feature.properties)).toStrictEqual([
      { dot: true, radius: 1.5, alpha: 0.55 },
      { dot: true, radius: 2.25, alpha: 0.95 },
      { dot: true, radius: 3, alpha: 0.95 },
    ]);
  });
});

describe(detectionLayers, () => {
  it("draws cells and dots in the detection tint, from tokens", () => {
    const [cells, dots] = detectionLayers();
    expect(cells?.id).toBe(DETECTION_CELLS);
    expect(dots?.id).toBe(DETECTION_DOTS);
    expect(cells?.paint?.["circle-color"]).toBe(kindColors.detection);
    expect(dots?.paint?.["circle-color"]).toBe(kindColors.detection);
    expect(dots?.paint?.["circle-stroke-color"]).toBe(colorTokens.canvas);
  });
});

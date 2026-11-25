import type { Map as MapLibreMap } from "maplibre-gl";
import { describe, expect, it, vi } from "vite-plus/test";

import { fitBox } from "./fitBox.ts";

const OPTIONS = { breathingRoom: 0, maxZoom: 8 } as const;

function framed(box: { west: number; south: number; east: number; north: number }): unknown {
  const fitBounds = vi.fn<MapLibreMap["fitBounds"]>();
  fitBox({ fitBounds }, box, OPTIONS);
  return fitBounds.mock.calls[0]?.[0];
}

describe(fitBox, () => {
  it("frames an ordinary box as it is", () => {
    expect(framed({ west: -75.6, south: -55.9, east: -66.9, north: -17.5 })).toStrictEqual([
      -75.6, -55.9, -66.9, -17.5,
    ]);
  });

  it("unwraps a box across ±180° so the camera takes the short way", () => {
    expect(framed({ west: 177, south: -21, east: -178, north: -12 })).toStrictEqual([177, -21, 182, -12]);
  });

  it("widens a single point to half a degree", () => {
    expect(framed({ west: 10, south: 40, east: 10, north: 40 })).toStrictEqual([9.75, 39.75, 10.25, 40.25]);
  });
});

import { describe, expect, it } from "vite-plus/test";

import {
  aircraftAllowed,
  aircraftCells,
  apiZoom,
  flatWorldZoom,
  globeZoom,
  severityShares,
  viewOf,
} from "./viewport.ts";

const WORLD = "-180,-85,180,85";

describe(viewOf, () => {
  it("rounds an ordinary view to two decimals", () => {
    expect(viewOf({ west: -9.50421, south: 36.951, east: -6.18901, north: 42.15 }, 6.4)).toStrictEqual({
      zoom: 7,
      bbox: "-9.5,36.95,-6.19,42.15",
    });
  });

  it("asks for the whole world when the view is half the globe or more", () => {
    expect(viewOf({ west: -100, south: -60, east: 80, north: 60 }, 1.4).bbox).toBe(WORLD);
    expect(viewOf({ west: -540, south: -90, east: 540, north: 90 }, 0).bbox).toBe(WORLD);
  });

  it("asks for the whole world when the bounds are not numbers", () => {
    expect(viewOf({ west: Number.NaN, south: 0, east: 10, north: 10 }, 3).bbox).toBe(WORLD);
    expect(viewOf({ west: 0, south: 10, east: 10, north: 0 }, 3).bbox).toBe(WORLD);
  });

  it("clamps latitudes to 85°", () => {
    expect(viewOf({ west: 0, south: 80, east: 20, north: 89 }, 4).bbox).toBe("0,80,20,85");
  });

  it("crosses the antimeridian with west greater than east", () => {
    // A wrapped MapLibre view over Fiji: 173…183 becomes 173…-177.
    expect(viewOf({ west: 173, south: -22, east: 183, north: -12 }, 5).bbox).toBe("173,-22,-177,-12");
    expect(viewOf({ west: -190, south: -22, east: -170, north: -12 }, 5).bbox).toBe("170,-22,-170,-12");
  });

  it("keeps a view ending on ±180° on its own side", () => {
    expect(viewOf({ west: 170, south: 0, east: 180, north: 10 }, 5).bbox).toBe("170,0,180,10");
    expect(viewOf({ west: 180, south: 0, east: 190, north: 10 }, 5).bbox).toBe("-180,0,-170,10");
  });

  it.each([
    [179.996, 179.999, "180,0,180,1"],
    [-179.999, -179.996, "-180,0,-180,1"],
    [179.999, 180.001, "180,0,-180,1"],
  ])("never widens a narrow view at ±180° (%f…%f) into the whole band", (west, east, bbox) => {
    expect(viewOf({ west, south: 0, east, north: 1 }, 22).bbox).toBe(bbox);
  });
});

describe(apiZoom, () => {
  it("steps up a level on each whole MapLibre zoom", () => {
    expect(apiZoom(4.99)).toBe(5);
    expect(apiZoom(5)).toBe(6);
  });

  it("is the 256 px pyramid's zoom, rounded down, as the apps and the server's grid", () => {
    expect(apiZoom(1.4)).toBe(2);
    expect(apiZoom(-2)).toBe(0);
    expect(apiZoom(25)).toBe(22);
    expect(apiZoom(Number.NaN)).toBe(0);
  });
});

describe(aircraftCells, () => {
  // backend internal/aircraft/aircraft_test.go, TestCells.
  it.each([
    ["inside one cell", "-9.3,38.6,-9,38.8", 1],
    ["iberia", "-9.9,36,-0.1,44", 4],
    ["across the antimeridian", "178,-18,-178,-16", 2],
    ["the pole clamps", "0,88,1,90", 1],
    ["a continent", "-10,35,30,60", 54],
    ["six cells", "-9.9,36,4.9,44", 6],
  ])("counts %s as the server does", (_name, bbox, cells) => {
    expect(aircraftCells(bbox)).toBe(cells);
  });

  it("refuses what params.BBox refuses", () => {
    expect(aircraftCells("1,2,3")).toBeUndefined();
    expect(aircraftCells("a,0,1,1")).toBeUndefined();
    expect(aircraftCells("0,,1,1")).toBeUndefined();
    expect(aircraftCells("0,10,1,5")).toBeUndefined();
    expect(aircraftCells("-181,0,1,1")).toBeUndefined();
  });
});

describe(aircraftAllowed, () => {
  it("allows four cells and no more", () => {
    expect(aircraftAllowed("-9.9,36,-0.1,44")).toBe(true);
    expect(aircraftAllowed("-9.9,36,4.9,44")).toBe(false);
    expect(aircraftAllowed(WORLD)).toBe(false);
    expect(aircraftAllowed("nonsense")).toBe(false);
  });

  it("allows what viewOf sends across the antimeridian", () => {
    expect(aircraftAllowed(viewOf({ west: 178, south: -18, east: 182, north: -16 }, 8).bbox)).toBe(true);
  });
});

describe(severityShares, () => {
  it("is most severe first and sums to one", () => {
    expect(severityShares({ severities: { minor: 3, severe: 1 } })).toStrictEqual([
      { severity: "severe", share: 0.25 },
      { severity: "minor", share: 0.75 },
    ]);
  });

  it("invents no split when the server sent none", () => {
    expect(severityShares({ severities: {} })).toStrictEqual([]);
  });
});

function diameterAt(zoom: number, latitude: number): number {
  return (512 / Math.PI) * Math.cos((latitude * Math.PI) / 180) * 2 ** zoom;
}

describe(globeZoom, () => {
  it("fills most of a phone's width", () => {
    const zoom = globeZoom({ width: 390, height: 743 }, 25);
    expect(diameterAt(zoom, 25)).toBeCloseTo(390 * 0.9);
  });

  it("leaves room around the globe on a wide screen", () => {
    const zoom = globeZoom({ width: 1155, height: 951 }, 25);
    expect(diameterAt(zoom, 25)).toBeCloseTo(951 * 0.62);
    expect(zoom).toBeCloseTo(2, 0);
  });

  it("stays in range for an empty or huge map", () => {
    expect(globeZoom({ width: 0, height: 0 }, 25)).toBe(0.5);
    expect(globeZoom({ width: 20_000, height: 20_000 }, 0)).toBe(3);
  });
});

describe(flatWorldZoom, () => {
  it("fits the world's width", () => {
    expect(flatWorldZoom(1024)).toBe(1);
    expect(flatWorldZoom(2048)).toBe(2);
  });

  it("never zooms out past the whole world", () => {
    expect(flatWorldZoom(390)).toBe(0);
    expect(flatWorldZoom(0)).toBe(0);
  });
});

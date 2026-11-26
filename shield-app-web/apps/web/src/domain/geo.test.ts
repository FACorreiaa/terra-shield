import { describe, expect, it } from "vite-plus/test";

import type { Coordinate } from "./geo.ts";
import { contains, crossesAntimeridian, eventShape, frame, geoBox, longitudeSpan, unwrapped } from "./geo.ts";

const ring = (longitudes: number[]): Coordinate[] => longitudes.map((longitude) => ({ latitude: 0, longitude }));
const unwrappedLons = (longitudes: number[], anchor?: number): number[] =>
  unwrapped(ring(longitudes), anchor).map((coordinate) => coordinate.longitude);
const at = (longitude: number, latitude: number): Coordinate => ({ latitude, longitude });

describe(geoBox, () => {
  it("reads the wire's four numbers", () => {
    expect(geoBox([-9.5, 36.95, -6.2, 42.15])).toStrictEqual({ west: -9.5, south: 36.95, east: -6.2, north: 42.15 });
    expect(geoBox([1, 2, 3])).toBeUndefined();
  });
});

describe(contains, () => {
  it("holds both sides of the antimeridian when the box crosses it", () => {
    const pacific = { west: 173, south: -22, east: -177, north: -12 };
    expect(crossesAntimeridian(pacific)).toBe(true);
    expect(longitudeSpan(pacific)).toBe(10);
    expect(contains(pacific, at(179.5, -18))).toBe(true);
    expect(contains(pacific, at(-178.5, -18))).toBe(true);
    expect(contains(pacific, at(0, -18))).toBe(false);
  });
});

describe(unwrapped, () => {
  it("keeps a ring across the antimeridian continuous", () => {
    expect(unwrappedLons([170, 179, -179, -170, 170])).toStrictEqual([170, 179, 181, 190, 170]);
    expect(unwrappedLons([-170, 175, -175])).toStrictEqual([-170, -185, -175]);
  });

  it("leaves a ring away from the antimeridian unchanged", () => {
    expect(unwrapped(ring([10, 20, 30, 10]))).toStrictEqual(ring([10, 20, 30, 10]));
    expect(unwrapped([])).toStrictEqual([]);
  });

  it("starts a hole near its outer ring", () => {
    // Outer ring unwrapped to 170…190; a hole written at -175 must sit at 185.
    expect(unwrappedLons([-175, -174, -175], 170)).toStrictEqual([185, 186, 185]);
  });
});

describe(frame, () => {
  it("gives a point a fixed span", () => {
    expect(frame({ kind: "point", point: at(20, 10) }, at(20, 10))).toStrictEqual({
      west: 19,
      south: 9,
      east: 21,
      north: 11,
    });
  });

  it("pads an area by a fifth each side", () => {
    const square = [at(0, 0), at(10, 0), at(10, 5), at(0, 5), at(0, 0)];
    expect(frame({ kind: "polygons", polygons: [[square]] }, at(5, 2.5))).toStrictEqual({
      west: -2,
      south: -1,
      east: 12,
      north: 6,
    });
  });

  it("keeps an area across the antimeridian one box", () => {
    const square = [at(170, 0), at(-170, 0), at(-170, 10), at(170, 10), at(170, 0)];
    const box = frame({ kind: "polygons", polygons: [[square]] }, at(180, 5));
    expect(crossesAntimeridian(box)).toBe(true);
    expect([box.west, box.east]).toStrictEqual([166, -166]);
  });
});

describe(eventShape, () => {
  it("reads each GeoJSON type", () => {
    expect(eventShape({ type: "Point", coordinates: [20, 10] })).toStrictEqual({ kind: "point", point: at(20, 10) });
    expect(
      eventShape({
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      }),
    ).toStrictEqual({
      kind: "lines",
      lines: [[at(0, 0), at(1, 1)]],
    });
    expect(
      eventShape({
        type: "MultiPolygon",
        coordinates: [
          [
            [
              [0, 0],
              [1, 0],
              [1, 1],
              [0, 0],
            ],
          ],
        ],
      }),
    ).toStrictEqual({ kind: "polygons", polygons: [[[at(0, 0), at(1, 0), at(1, 1), at(0, 0)]]] });
  });

  it("draws no half shapes and no point sets", () => {
    expect(eventShape({ type: "MultiPoint", coordinates: [[0, 0]] })).toBeUndefined();
    expect(eventShape({ type: "LineString", coordinates: [[0, 0]] })).toBeUndefined();
    expect(
      eventShape({
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 1],
          ],
          [[0, "x"]],
        ],
      }),
    ).toBeUndefined();
  });
});

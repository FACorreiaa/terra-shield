import { describe, expect, it } from "vite-plus/test";

import { clusterArcs, clusterCount, clusterLabel, clusterSize } from "./clusterArt.ts";

describe(clusterSize, () => {
  it("is 36 px for two events and stops at 56 px", () => {
    expect(clusterSize(2)).toBe(36);
    expect(clusterSize(1000)).toBe(56);
    expect(clusterSize(100_000)).toBe(56);
    expect(clusterSize(20)).toBeGreaterThan(36);
    expect(clusterSize(20)).toBeLessThan(56);
  });
});

describe(clusterArcs, () => {
  it("draws no arc when there is one severity or no split", () => {
    expect(clusterArcs([], 40)).toStrictEqual([]);
    expect(clusterArcs([{ severity: "minor", share: 1 }], 40)).toStrictEqual([]);
  });

  it("draws one arc per severity in the given order, starting at twelve o'clock", () => {
    const arcs = clusterArcs(
      [
        { severity: "severe", share: 0.25 },
        { severity: "minor", share: 0.75 },
      ],
      40,
    );
    expect(arcs.map((arc) => arc.severity)).toStrictEqual(["severe", "minor"]);
    expect(arcs[0]?.path.startsWith("M 20.00 7.00")).toBe(true);
    expect(arcs[1]?.path).toContain(" 0 1 1 ");
  });
});

describe(clusterCount, () => {
  it("compacts large counts", () => {
    expect(clusterCount(77)).toBe("77");
    expect(clusterCount(1240)).toBe("1.2K");
  });
});

describe(clusterLabel, () => {
  it("names the count and the highest severity", () => {
    expect(clusterLabel(14, "extreme")).toBe("Cluster, 14 events, highest severity extreme");
  });
});

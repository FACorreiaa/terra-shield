import type { EventCluster, EventKind, EventList, HazardEvent, Severity } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import { cellDescription, compactWords, liveMapCells } from "./statStrip.ts";

const NOW = "2027-01-15T08:00:00Z";

function event(severity: Severity, kind: EventKind = "observed", resolved = false): HazardEvent {
  return {
    id: "e",
    source: "test",
    type: "flood",
    kind,
    status: resolved ? "resolved" : "active",
    severity,
    title: "e",
    centroid: { latitude: 0, longitude: 0 },
    countries: [],
    observedAt: NOW,
    updatedAt: NOW,
    fetchedAt: NOW,
    attrs: {},
  };
}

function cluster(count: number, maxSeverity: Severity, severities: Record<string, number> = {}): EventCluster {
  return {
    centroid: { latitude: 0, longitude: 0 },
    count,
    maxSeverity,
    types: { flood: count },
    severities,
    statuses: { active: count },
    activeSeverities: severities,
    bbox: [0, 0, 1, 1],
  };
}

function list(events: HazardEvent[], clusters: EventCluster[] = [], truncated = false): EventList {
  return { events, clusters, truncated, generatedAt: NOW };
}

const values = (cells: { value: string }[]): string[] => cells.map((cell) => cell.value);
const clusterOnly = (only: EventCluster): string[] => values(liveMapCells(list([], [only])));

describe(liveMapCells, () => {
  it("shows dashes and no filters before the first response", () => {
    expect(liveMapCells(undefined)).toStrictEqual([
      { value: "–", label: "Observed events" },
      { value: "–", label: "High severity" },
      { value: "–", label: "Official warnings" },
    ]);
  });

  it("counts single events exactly, with the high severity and warnings filters", () => {
    const cells = liveMapCells(list([event("extreme", "warning"), event("severe"), event("minor", "warning")]));
    expect(cells).toStrictEqual([
      { value: "3", label: "Observed events" },
      { value: "2", label: "High severity", filter: "highSeverity" },
      { value: "2", label: "Official warnings", filter: "warnings" },
    ]);
  });

  it("adds cluster members, from their severity split", () => {
    const cells = liveMapCells(
      list([event("severe", "warning")], [cluster(10, "extreme", { minor: 7, severe: 2, extreme: 1 })]),
    );
    // Clusters carry no kind: warnings are at least the single ones.
    expect(values(cells)).toStrictEqual(["11", "4", "1+"]);
  });

  it("counts a cluster without a split once, as a lower bound when it holds more", () => {
    expect(clusterOnly(cluster(5, "severe"))).toStrictEqual(["5", "1+", "–"]);
    expect(clusterOnly(cluster(1, "extreme"))).toStrictEqual(["1", "1", "–"]);
    expect(clusterOnly(cluster(5, "moderate"))).toStrictEqual(["5", "0", "–"]);
  });

  it("marks every count short when the server truncated", () => {
    const truncated = list([event("severe", "warning")], [], true);
    expect(values(liveMapCells(truncated))).toStrictEqual(["1+", "1+", "1+"]);
  });

  it("never counts resolved events", () => {
    const cells = liveMapCells(list([event("extreme", "warning", true), event("minor")]), true);
    expect(values(cells)).toStrictEqual(["1", "0", "0"]);
  });

  it("shows dashes for counts that need clusters while resolved events are included", () => {
    const withClusters = list([event("severe", "warning")], [cluster(4, "severe", { severe: 4 })]);
    expect(values(liveMapCells(withClusters, true))).toStrictEqual(["–", "–", "1+"]);
    expect(values(liveMapCells(withClusters))).toStrictEqual(["5", "5", "1+"]);
  });
});

describe(compactWords, () => {
  it("is singular only for exactly one", () => {
    expect(compactWords({ value: "1", label: "Official warnings" })).toBe("warning");
    expect(compactWords({ value: "1+", label: "Official warnings" })).toBe("warnings");
    expect(compactWords({ value: "4", label: "Observed events" })).toBe("events");
    expect(compactWords({ value: "–", label: "High severity" })).toBe("high severity");
  });
});

describe(cellDescription, () => {
  it("reads lower bounds and unknowns in words", () => {
    expect(cellDescription({ value: "3", label: "High severity" })).toBe("High severity: 3");
    expect(cellDescription({ value: "12+", label: "Official warnings" })).toBe("Official warnings: at least 12");
    expect(cellDescription({ value: "–", label: "Official warnings" })).toBe(
      "Official warnings: not counted at this zoom",
    );
  });
});

import type { EventCluster } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import { conflictCount, conflictHeadline, conflictSectionNote } from "./conflicts.ts";
import { makeEvent } from "./testEvents.ts";

function cluster(count: number): EventCluster {
  return {
    centroid: { longitude: 30, latitude: 15 },
    count,
    maxSeverity: "severe",
    types: { conflict: count },
    severities: { severe: count },
    statuses: {},
    activeSeverities: {},
    bbox: [29, 14, 31, 16],
  };
}

const TWO = [makeEvent({ id: "a", type: "conflict" }), makeEvent({ id: "b", type: "conflict" })];

describe(conflictCount, () => {
  it("counts the listed events and every cluster's members", () => {
    expect(conflictCount({ events: TWO, clusters: [cluster(7), cluster(3)] })).toBe(12);
  });
});

describe(conflictHeadline, () => {
  it("says how many are in view, in the apps' words", () => {
    expect(conflictHeadline({ list: { events: TWO, clusters: [cluster(7)], truncated: false }, failed: false })).toBe(
      "Armed conflicts, past 90 days · 9 in this view",
    );
  });

  it("says none, and asks to zoom in when the answer was cut off", () => {
    expect(conflictHeadline({ list: { events: [], clusters: [], truncated: false }, failed: false })).toBe(
      "Armed conflicts, past 90 days · none in this view",
    );
    expect(conflictHeadline({ list: { events: TWO, clusters: [], truncated: true }, failed: false })).toBe(
      "Armed conflicts, past 90 days · 2 in this view · zoom in for all",
    );
  });

  it("says loading before the first answer, and when it didn't load", () => {
    expect(conflictHeadline({ list: undefined, failed: false })).toBe("Armed conflicts · loading…");
    expect(conflictHeadline({ list: { events: TWO, clusters: [], truncated: false }, failed: true })).toBe(
      "Armed conflicts didn’t load",
    );
  });
});

describe(conflictSectionNote, () => {
  it("says what the data are, and how many the map grouped", () => {
    expect(conflictSectionNote(0)).toBe("Monthly research estimates (UCDP), not live reports.");
    expect(conflictSectionNote(14)).toBe(
      "Monthly research estimates (UCDP). 14 more grouped on the map; zoom in to list them.",
    );
  });
});

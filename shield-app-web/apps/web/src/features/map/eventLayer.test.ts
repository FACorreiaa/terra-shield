import { describe, expect, it } from "vite-plus/test";

import { makeEvent } from "@/domain/testEvents.ts";

import { eventFeatures, markerRequests } from "./eventLayer.ts";

const STALE_UCDP = new Set(["ucdp", "usgs"]);
const QUAKE = makeEvent({ id: "quake", type: "earthquake", severity: "minor", source: "usgs" });
const EXACT = makeEvent({
  id: "exact",
  type: "conflict",
  severity: "extreme",
  source: "ucdp",
  attrs: { locationPrecision: "exact" },
});
const ROUGH = makeEvent({
  id: "rough",
  type: "conflict",
  severity: "severe",
  source: "ucdp",
  attrs: { locationPrecision: "admin1" },
});

describe(eventFeatures, () => {
  it("draws conflicts with their own glyph, never stale, under every hazard", () => {
    const features = eventFeatures([QUAKE, EXACT, ROUGH], STALE_UCDP).features.map((feature) => feature.properties);
    expect(features.map((each) => each.image)).toStrictEqual([
      "marker/earthquake/minor/observed/stale",
      "marker/conflict/extreme/observed/live",
      "marker/conflict/severe/observed/live/approximate",
    ]);
    // An extreme conflict still sorts under a minor (stale) hazard.
    expect(features.map((each) => each.priority)).toStrictEqual([10, -7, -8]);
  });

  it("leaves heat detections to their own layer", () => {
    const heat = makeEvent({ id: "heat", kind: "detection" });
    expect(eventFeatures([heat], new Set()).features).toStrictEqual([]);
  });
});

describe(markerRequests, () => {
  it("asks for an approximate conflict's image apart from an exact one's", () => {
    const rough = { ...ROUGH, severity: "extreme" } as const;
    expect(markerRequests([EXACT, rough], new Set())).toHaveLength(2);
  });
});

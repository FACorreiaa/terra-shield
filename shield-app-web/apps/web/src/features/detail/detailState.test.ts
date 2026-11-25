import { describe, expect, it } from "vite-plus/test";

import { makeEvent } from "@/domain/testEvents.ts";

import type { EventDetail } from "./detailState.ts";
import { detailState } from "./detailState.ts";

const event = makeEvent();
const detail: EventDetail = {
  ...event,
  geometry: { type: "Point", coordinates: [event.centroid.longitude, event.centroid.latitude] },
  sourceInfo: {
    id: "usgs",
    name: "USGS",
    url: "https://earthquake.usgs.gov",
    license: "Public domain",
    attribution: "U.S. Geological Survey",
    pollIntervalSeconds: 60,
    enabled: true,
  },
  revisions: [],
};

describe(detailState, () => {
  it("loads until the first answer", () => {
    expect(detailState(undefined, false, undefined)).toStrictEqual({ kind: "loading" });
  });

  it("shows the detail, and keeps it when a refresh fails", () => {
    expect(detailState({ found: true, detail }, false, event)).toStrictEqual({ kind: "loaded", detail });
    expect(detailState({ found: true, detail }, true, event)).toStrictEqual({ kind: "loaded", detail });
  });

  it("says gone with the event as last seen, or with nothing for a link never seen", () => {
    expect(detailState({ found: false }, false, event)).toStrictEqual({ kind: "gone", last: event });
    expect(detailState({ found: false }, false, undefined)).toStrictEqual({ kind: "gone", last: undefined });
  });

  it("fails only when nothing ever loaded", () => {
    expect(detailState(undefined, true, event)).toStrictEqual({ kind: "failed" });
  });
});

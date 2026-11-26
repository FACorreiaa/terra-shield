import { describe, expect, it } from "vite-plus/test";

import { liveChange } from "./liveChange.ts";

const EVENT = {
  id: "6f1c2a7e-0000-4000-8000-000000000001",
  source: "usgs",
  type: "earthquake",
  kind: "observed",
  status: "active",
  severity: "severe",
  title: "M 6.1 - Chile",
  centroid: { latitude: -30.2, longitude: -71.6 },
  countries: [],
  observedAt: "2026-11-26T10:00:00Z",
  updatedAt: "2026-11-26T10:00:00Z",
  fetchedAt: "2026-11-26T10:00:00Z",
  attrs: {},
};

function message(event: string, data: unknown): { id: string; event: string; data: string } {
  return { id: "9", event, data: typeof data === "string" ? data : JSON.stringify(data) };
}

describe(liveChange, () => {
  it("decodes each kind of event change", () => {
    expect(liveChange(message("event.created", EVENT))).toStrictEqual({ kind: "created", event: EVENT });
    expect(liveChange(message("event.updated", EVENT))).toStrictEqual({ kind: "upsert", event: EVENT });
    expect(liveChange(message("event.resolved", EVENT))).toStrictEqual({ kind: "upsert", event: EVENT });
    expect(liveChange(message("event.removed", { id: EVENT.id }))).toStrictEqual({ kind: "removed", id: EVENT.id });
  });

  it("decodes the stream's own signals", () => {
    expect(liveChange(message("reset", "{}"))).toStrictEqual({ kind: "reset" });
    expect(liveChange(message("ready", "{}"))).toStrictEqual({ kind: "ready" });
  });

  it.each([
    ["event.updated", "{not json"],
    ["event.created", { ...EVENT, centroid: undefined }],
    ["event.removed", {}],
    ["toString", "{}"],
    ["message", "{}"],
  ])("skips %s it can't read", (name, data) => {
    expect(liveChange(message(name, data))).toBeUndefined();
  });
});

import type { EventList, HazardEvent } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import { DEFAULT_QUERY } from "./filters.ts";
import type { PatchContext } from "./livePatch.ts";
import { burstMeter, isNews, patchEvents } from "./livePatch.ts";

const NOW = new Date("2026-11-26T12:00:00Z");
const EUROPE = "-25,34,45,72";

function event(id: string, overrides: Partial<HazardEvent> = {}): HazardEvent {
  return {
    id,
    source: "usgs",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity: "moderate",
    title: `Event ${id}`,
    centroid: { latitude: 48, longitude: 2 },
    countries: [],
    observedAt: "2026-11-26T11:00:00Z",
    updatedAt: "2026-11-26T11:00:00Z",
    fetchedAt: "2026-11-26T11:00:00Z",
    attrs: {},
    ...overrides,
  };
}

function list(events: HazardEvent[], extra: Partial<EventList> = {}): EventList {
  return { events, clusters: [], truncated: false, generatedAt: NOW.toISOString(), ...extra };
}

function context(overrides: Partial<PatchContext> = {}): PatchContext {
  return { view: { zoom: 7, bbox: EUROPE }, query: DEFAULT_QUERY, now: NOW, inBurst: () => false, ...overrides };
}

const SYDNEY = { latitude: -33, longitude: 151 };
const shown = list([event("a"), event("b")]);

/** The ids after an upsert, or what happened instead ("none", "reload"). */
function afterUpsert(
  target: EventList,
  changed: HazardEvent,
  overrides: Partial<PatchContext> = {},
): string[] | string {
  const outcome = patchEvents(target, { kind: "upsert", event: changed }, context(overrides));
  return outcome.kind === "patched"
    ? outcome.list.events.map((each) => `${each.id}${each.title === "Revised" ? "*" : ""}`)
    : outcome.kind;
}

function afterRemoval(target: EventList, id: string, overrides: Partial<PatchContext> = {}): string[] | string {
  const outcome = patchEvents(target, { kind: "removed", id }, context(overrides));
  return outcome.kind === "patched" ? outcome.list.events.map((each) => each.id) : outcome.kind;
}

const REVISED = event("a", { title: "Revised" });
const FRESH = event("c");
const RESOLVED = event("a", { status: "resolved", endedAt: "2026-11-26T11:30:00Z" });
const MOVED = event("a", { centroid: SYDNEY });
const FAR = event("c", { centroid: SYDNEY });
const OLD = event("c", { updatedAt: "2026-11-23T12:00:00Z" });
const MINOR = event("c", { severity: "minor" });
const FORECAST = event("c", { kind: "forecast" });
const LOW: Partial<PatchContext> = { view: { zoom: 5, bbox: EUROPE } };
const CUT = list([event("a")], { truncated: true });
const CLUSTER: EventList["clusters"][number] = {
  centroid: SYDNEY,
  count: 3,
  maxSeverity: "minor",
  types: {},
  severities: {},
  statuses: {},
  activeSeverities: {},
  bbox: [150, -34, 152, -32],
};
const CLUSTERED = list([event("a")], { clusters: [CLUSTER] });
const BURST: Partial<PatchContext> = { inBurst: () => true };

describe(patchEvents, () => {
  it("patches from zoom 6: a change in place, a new event in, a removal out", () => {
    expect(afterUpsert(shown, REVISED)).toStrictEqual(["b", "a*"]);
    expect(afterUpsert(shown, FRESH)).toStrictEqual(["a", "b", "c"]);
    expect(afterRemoval(shown, "b")).toStrictEqual(["a"]);
  });

  it("takes out a shown event that stops matching or moves out of view", () => {
    expect(afterUpsert(shown, RESOLVED)).toStrictEqual(["b"]);
    expect(afterUpsert(shown, RESOLVED, { query: { ...DEFAULT_QUERY, includeResolved: true } })).toStrictEqual([
      "b",
      "a",
    ]);
    expect(afterUpsert(shown, MOVED)).toStrictEqual(["b"]);
  });

  it("leaves the map alone for a change out of view, and adds nothing filtered out", () => {
    expect(afterUpsert(shown, FAR)).toBe("none");
    expect(afterUpsert(shown, OLD)).toStrictEqual(["a", "b"]);
    expect(afterUpsert(shown, MINOR, { query: { ...DEFAULT_QUERY, minimumSeverity: "severe" } })).toStrictEqual([
      "a",
      "b",
    ]);
  });

  it("reloads below zoom 6, or when the server cut the list", () => {
    expect(afterUpsert(shown, FRESH, LOW)).toBe("reload");
    expect(afterUpsert(CUT, FRESH)).toBe("reload");
    expect(afterRemoval(CUT, "z")).toBe("reload");
  });

  it("reloads a removal below zoom 6 only when it may matter", () => {
    expect(afterRemoval(shown, "a", LOW)).toBe("reload");
    expect(afterRemoval(shown, "z", LOW)).toBe("none");
    expect(afterRemoval(CLUSTERED, "z", LOW)).toBe("reload");
  });

  it("folds a burst into one reload", () => {
    expect(afterUpsert(shown, FRESH, BURST)).toBe("reload");
    expect(afterRemoval(shown, "a", BURST)).toBe("reload");
  });
});

describe(burstMeter, () => {
  it("is true past 50 patches in a second, and resets after", () => {
    const clock = { at: 0 };
    const inBurst = burstMeter(() => clock.at);
    const first = Array.from({ length: 51 }, () => inBurst());
    expect(first.filter(Boolean)).toHaveLength(1);
    clock.at = 1001;
    expect(inBurst()).toBe(false);
  });
});

describe(isNews, () => {
  it("is news in view and matching the filters", () => {
    expect(isNews(FRESH, context())).toBe(true);
    expect(isNews(FAR, context())).toBe(false);
    const observedOnly = context({ query: { ...DEFAULT_QUERY, kinds: new Set(["observed"] as const) } });
    expect(isNews(FORECAST, observedOnly)).toBe(false);
  });
});

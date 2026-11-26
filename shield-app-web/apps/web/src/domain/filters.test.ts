import { describe, expect, it } from "vite-plus/test";

import type { EventQuery } from "./filters.ts";
import {
  activeFilters,
  CONFLICTS_QUERY,
  DEFAULT_QUERY,
  filterLabel,
  isDefault,
  isOn,
  matches,
  removing,
  toApiQuery,
  toggling,
  windowLabel,
} from "./filters.ts";
import { AT, makeEvent, SAMPLE_NOW, sampleEvents } from "./testEvents.ts";

const query = (overrides: Partial<EventQuery>): EventQuery => ({ ...DEFAULT_QUERY, ...overrides });

describe(activeFilters, () => {
  it("finds none in the default, where the country is a place, not a filter", () => {
    expect(isDefault(DEFAULT_QUERY)).toBe(true);
    expect(isDefault(query({ country: "PT" }))).toBe(true);
  });

  it("lists the dimensions that differ, in chip order", () => {
    const changed = query({
      types: new Set(["earthquake"]),
      minimumSeverity: "severe",
      kinds: new Set(["warning"]),
      windowSeconds: 7 * 86_400,
    });
    expect(activeFilters(changed)).toStrictEqual(["types", "severity", "kinds", "window"]);
    expect(activeFilters(changed).map((filter) => filterLabel(changed, filter))).toStrictEqual([
      "Earthquake",
      "Severe+",
      "Warnings only",
      "7 d",
    ]);
  });

  it("makes include resolved a chip with its own reset", () => {
    const resolved = query({ includeResolved: true });
    expect(activeFilters(resolved)).toStrictEqual(["status"]);
    expect(filterLabel(resolved, "status")).toBe("With resolved");
    expect(isDefault(removing(resolved, "status"))).toBe(true);
  });
});

describe(filterLabel, () => {
  it.each<[Partial<EventQuery>, Parameters<typeof filterLabel>[1], string]>([
    [{ types: new Set(["storm", "flood"]) }, "types", "Flood, Storm"],
    [{ types: new Set(["flood", "storm", "wildfire"]) }, "types", "3 types"],
    [{}, "types", "14 types"],
    [{ kinds: new Set(["observed", "forecast"]) }, "kinds", "Observed + forecasts"],
    [{ windowSeconds: 3600 }, "window", "1 h"],
    [{ windowSeconds: undefined }, "window", "Any time"],
    [{}, "status", "Active only"],
  ])("reads %o's %s chip as %s", (overrides, filter, expected) => {
    expect(filterLabel(query(overrides), filter)).toBe(expected);
  });
});

describe(windowLabel, () => {
  it.each([
    [3600, "1 h"],
    [21_600, "6 h"],
    [86_400, "24 h"],
    [604_800, "7 d"],
  ])("reads %i s as %s", (seconds, expected) => {
    expect(windowLabel(seconds)).toBe(expected);
  });
});

describe(removing, () => {
  it("resets one dimension", () => {
    const changed = query({ minimumSeverity: "extreme", windowSeconds: 3600 });
    const reset = removing(changed, "severity");
    expect(reset.minimumSeverity).toBe("minor");
    expect(reset.windowSeconds).toBe(3600);
    expect(isDefault(removing(reset, "window"))).toBe(true);
  });
});

describe(toggling, () => {
  it("filters on a cell tap and undoes on a second", () => {
    const high = toggling(DEFAULT_QUERY, "highSeverity");
    expect(high.minimumSeverity).toBe("severe");
    expect(isOn(high, "highSeverity")).toBe(true);
    expect(isDefault(toggling(high, "highSeverity"))).toBe(true);
  });

  it("touches only its own dimension", () => {
    const warnings = toggling(DEFAULT_QUERY, "warnings");
    expect([...warnings.kinds]).toStrictEqual(["warning"]);
    expect(isOn(warnings, "warnings")).toBe(true);
    expect(isDefault(toggling(warnings, "warnings"))).toBe(true);
    expect(toggling(toggling(DEFAULT_QUERY, "highSeverity"), "warnings").minimumSeverity).toBe("severe");
  });
});

const hoursFromAt = (hours: number): string => new Date(AT.getTime() + hours * 3_600_000).toISOString();
const windowed = (updatedHours: number, endedHours?: number): ReturnType<typeof makeEvent> =>
  makeEvent({
    source: "meteoalarm",
    type: "wind",
    kind: "warning",
    status: endedHours === undefined ? "active" : "resolved",
    severity: "moderate",
    updatedAt: hoursFromAt(updatedHours),
    ...(endedHours === undefined ? {} : { endedAt: hoursFromAt(endedHours) }),
  });

describe(matches, () => {
  const withResolved = query({ includeResolved: true });

  it("windows a resolved event by its end (PR #23)", () => {
    expect(matches(withResolved, windowed(-48, -1), AT)).toBe(true);
    expect(matches(withResolved, windowed(-1, -48), AT)).toBe(false);
  });

  it("windows an active event by its update", () => {
    expect(matches(withResolved, windowed(-1), AT)).toBe(true);
    expect(matches(withResolved, windowed(-48), AT)).toBe(false);
  });

  it("hides resolved events and conflicts by default", () => {
    expect(sampleEvents().filter((event) => matches(DEFAULT_QUERY, event, SAMPLE_NOW))).toHaveLength(6);
    expect(matches(DEFAULT_QUERY, makeEvent({ type: "conflict" }), AT)).toBe(false);
    expect(matches(CONFLICTS_QUERY, makeEvent({ type: "conflict", updatedAt: "2026-09-10T00:00:00Z" }), AT)).toBe(true);
  });

  it("filters by severity, country and kind, and reads an unknown type as other", () => {
    const events = sampleEvents();
    const titles = (filter: EventQuery): string[] =>
      events.filter((event) => matches(filter, event, SAMPLE_NOW)).map((event) => event.title);
    expect(titles(query({ country: "PT" }))).toStrictEqual(["Wildfire near Leiria", "Heavy rain forecast, Minho"]);
    expect(titles(query({ minimumSeverity: "severe" }))).toHaveLength(3);
    const noForecasts = query({ kinds: new Set(["observed", "warning"]) });
    expect(titles(noForecasts)).not.toContain("Heavy rain forecast, Minho");
    const other = query({ types: new Set(["other"]) });
    expect(matches(other, makeEvent({ type: "meteor" }), AT)).toBe(true);
  });
});

describe(toApiQuery, () => {
  it("asks for the default with no type, so types added later still show", () => {
    expect(toApiQuery(DEFAULT_QUERY, AT)).toStrictEqual({
      kind: "forecast,observed,warning",
      status: "active",
      since: "2026-11-15T14:20:00.000Z",
    });
  });

  it("sends each chosen dimension, sorted", () => {
    const chosen = query({
      types: new Set(["tsunami", "earthquake"]),
      minimumSeverity: "severe",
      kinds: new Set(["warning"]),
      includeResolved: true,
      windowSeconds: 3600,
      country: "PT",
    });
    expect(toApiQuery(chosen, AT)).toStrictEqual({
      type: "earthquake,tsunami",
      severity: "extreme,severe",
      kind: "warning",
      status: "active,resolved",
      country: "PT",
      since: "2026-11-16T13:20:00.000Z",
    });
  });

  it("names conflicts for their layer, at any age", () => {
    expect(toApiQuery(CONFLICTS_QUERY, AT)).toStrictEqual({ type: "conflict", kind: "observed", status: "active" });
  });
});

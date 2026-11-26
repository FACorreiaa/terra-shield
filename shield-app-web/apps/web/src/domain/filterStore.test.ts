import { describe, expect, it } from "vite-plus/test";

import type { EventQuery } from "./filters.ts";
import { DEFAULT_QUERY } from "./filters.ts";
import { decodeFilters, DEFAULT_SAVED, encodeFilters } from "./filterStore.ts";
import { HAZARD_TYPES } from "./taxonomy.ts";

/** shield-core FilterStoreTests: what is excluded is stored, so new types show by default. */

const query = (overrides: Partial<EventQuery>): EventQuery => ({ ...DEFAULT_QUERY, ...overrides });

describe(encodeFilters, () => {
  it("round-trips", () => {
    const saved = {
      query: query({
        types: new Set(["earthquake", "tsunami"]),
        minimumSeverity: "moderate",
        kinds: new Set(["warning", "forecast"]),
        windowSeconds: 6 * 3600,
        includeResolved: true,
      }),
      showsDetections: true,
      showsConflicts: true,
    } as const;
    expect(decodeFilters(encodeFilters(saved))).toStrictEqual(saved);
  });

  it("stores what is excluded, never conflicts", () => {
    const blob: unknown = JSON.parse(encodeFilters(DEFAULT_SAVED));
    expect(blob).toStrictEqual({
      version: 1,
      excludedTypes: [],
      minimumSeverity: "minor",
      excludedKinds: [],
      window: 86_400,
      showsDetections: false,
      includeResolved: false,
      showsConflicts: false,
    });
    expect(decodeFilters(encodeFilters(DEFAULT_SAVED)).query.types.has("conflict")).toBe(false);
  });

  it("leaves the window out for any time", () => {
    const blob: unknown = JSON.parse(encodeFilters({ ...DEFAULT_SAVED, query: query({ windowSeconds: undefined }) }));
    expect(blob).not.toHaveProperty("window");
  });
});

describe(decodeFilters, () => {
  it("shows a type the save did not know", () => {
    const old =
      '{"version":1,"excludedTypes":["flood","no_such_type"],"minimumSeverity":"minor","excludedKinds":[],"window":86400,"showsDetections":false}';
    const { types } = decodeFilters(old).query;
    expect(types.has("flood")).toBe(false);
    expect(types.size).toBe(HAZARD_TYPES.length - 1);
  });

  it.each([undefined, "", "not json", '{"version":2}', '{"version":1}', "[]"])("gives the defaults for %o", (raw) => {
    expect(decodeFilters(raw)).toStrictEqual(DEFAULT_SAVED);
  });

  it("never restores a query that shows nothing", () => {
    const all = JSON.stringify(HAZARD_TYPES);
    const blob = `{"version":1,"excludedTypes":${all},"minimumSeverity":"bogus","excludedKinds":["observed","warning","forecast"],"window":null,"showsDetections":false}`;
    const decoded = decodeFilters(blob).query;
    expect(decoded.types).toStrictEqual(DEFAULT_QUERY.types);
    expect(decoded.kinds).toStrictEqual(DEFAULT_QUERY.kinds);
    expect(decoded.minimumSeverity).toBe("minor");
    // null isn't a window the sheet offers.
    expect(decoded.windowSeconds).toBe(86_400);
  });

  it("refuses a window the sheet doesn't offer", () => {
    const blob =
      '{"version":1,"excludedTypes":[],"minimumSeverity":"minor","excludedKinds":[],"window":1234,"showsDetections":false}';
    expect(decodeFilters(blob).query.windowSeconds).toBe(86_400);
  });

  it("keeps a version 1 blob's filters without its status", () => {
    const blob =
      '{"version":1,"excludedTypes":["flood"],"minimumSeverity":"severe","excludedKinds":[],"window":3600,"showsDetections":true}';
    const decoded = decodeFilters(blob);
    const types = new Set(HAZARD_TYPES.filter((type) => type !== "flood"));
    expect(decoded.query).toStrictEqual(query({ types, minimumSeverity: "severe", windowSeconds: 3600 }));
    expect(decoded.showsDetections).toBe(true);
    // Saved before the conflicts layer: off.
    expect(decoded.showsConflicts).toBe(false);
  });
});

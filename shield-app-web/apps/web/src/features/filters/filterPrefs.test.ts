import { afterEach, beforeEach, describe, expect, it } from "vite-plus/test";

import { DEFAULT_QUERY } from "@/domain/filters.ts";

import {
  resetFilterPrefsForTests,
  resetFilters,
  savedFilters,
  setQuery,
  setShowsDetections,
  toggleShowsConflicts,
} from "./filterPrefs.ts";

const KEY = "shield.filters";

/** The stored blob, parsed. */
function stored(): unknown {
  return JSON.parse(localStorage.getItem(KEY) ?? "{}");
}

describe("filter prefs", () => {
  beforeEach(() => {
    localStorage.clear();
    resetFilterPrefsForTests();
  });

  afterEach(() => {
    localStorage.clear();
    resetFilterPrefsForTests();
  });

  it("loads the defaults from nothing, and from a corrupt value", () => {
    expect(savedFilters().query).toStrictEqual(DEFAULT_QUERY);
    localStorage.setItem(KEY, "{not json");
    resetFilterPrefsForTests();
    expect(savedFilters().query).toStrictEqual(DEFAULT_QUERY);
  });

  it("keeps a change across visits in the apps' blob", () => {
    setQuery({ ...DEFAULT_QUERY, minimumSeverity: "severe" });
    resetFilterPrefsForTests();
    expect(savedFilters().query.minimumSeverity).toBe("severe");
    expect(stored()).toMatchObject({ version: 1, minimumSeverity: "severe" });
  });

  it("leaves the heat layer's switch as it was stored", () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({
        version: 1,
        excludedTypes: [],
        minimumSeverity: "minor",
        excludedKinds: [],
        window: 86_400,
        showsDetections: true,
      }),
    );
    resetFilterPrefsForTests();
    setQuery({ ...DEFAULT_QUERY, includeResolved: true });
    expect(stored()).toMatchObject({
      showsDetections: true,
      includeResolved: true,
    });
  });

  it("keeps the heat switch and the query apart, and Reset turns the heat layer off", () => {
    setQuery({ ...DEFAULT_QUERY, minimumSeverity: "severe" });
    setShowsDetections(true);
    resetFilterPrefsForTests();
    expect(savedFilters().showsDetections).toBe(true);
    expect(savedFilters().query.minimumSeverity).toBe("severe");

    resetFilters(DEFAULT_QUERY);
    expect(savedFilters()).toStrictEqual({ query: DEFAULT_QUERY, showsDetections: false, showsConflicts: false });
    expect(stored()).toMatchObject({ showsDetections: false, minimumSeverity: "minor" });
  });

  it("flips the conflicts layer from what is stored, and Reset leaves it on", () => {
    toggleShowsConflicts();
    expect(savedFilters().showsConflicts).toBe(true);
    resetFilters(DEFAULT_QUERY);
    expect(stored()).toMatchObject({ showsConflicts: true, showsDetections: false });
    toggleShowsConflicts();
    expect(savedFilters().showsConflicts).toBe(false);
  });
});

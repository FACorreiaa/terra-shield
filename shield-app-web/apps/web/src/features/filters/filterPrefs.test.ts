import { afterEach, beforeEach, describe, expect, it } from "vite-plus/test";

import { DEFAULT_QUERY } from "@/domain/filters.ts";

import { resetFilterPrefsForTests, savedFilters, setQuery } from "./filterPrefs.ts";

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
});

import { describe, expect, it } from "vite-plus/test";

import type { CountrySummary } from "@/domain/countries.ts";

import { countryState } from "./countryState.ts";

const SUMMARY: CountrySummary = {
  country: { iso2: "PT", iso3: "PRT", name: "Portugal", activeEvents: 0, bbox: [0, 0, 1, 1] },
  counts: [],
  coverage: { officialWarnings: true, sources: [] },
  events: [],
  recentChanges: [],
  generatedAt: "2027-01-15T08:00:00Z",
};

describe(countryState, () => {
  it("has no boundary for a malformed code or a 404", () => {
    expect(countryState(undefined, undefined, false)).toStrictEqual({ kind: "notFound" });
    expect(countryState("ZZ", { found: false }, false)).toStrictEqual({ kind: "notFound" });
  });

  it("loads, then fails only when nothing loaded before", () => {
    expect(countryState("PT", undefined, false)).toStrictEqual({ kind: "loading" });
    expect(countryState("PT", undefined, true)).toStrictEqual({ kind: "failed" });
  });

  it("keeps a loaded summary through a failed refresh", () => {
    expect(countryState("PT", { found: true, summary: SUMMARY }, true)).toStrictEqual({
      kind: "loaded",
      summary: SUMMARY,
      refreshFailed: true,
    });
  });
});

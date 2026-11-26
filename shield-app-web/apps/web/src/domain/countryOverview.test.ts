import type { HazardEvent, Severity } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import type { Country, CountrySummary } from "./countries.ts";
import { activeWords, capNote, cellWords, countryCells, directory } from "./countryOverview.ts";
import { makeEvent } from "./testEvents.ts";

const UNIT_BOX = [0, 0, 1, 1];

function country(iso2: string, name: string, active?: [count: number, worst: Severity]): Country {
  const base = { iso2, iso3: `${iso2}X`, name, activeEvents: 0, bbox: UNIT_BOX };
  return active === undefined ? base : { ...base, activeEvents: active[0], maxSeverity: active[1] };
}

const ALL = [
  country("PT", "Portugal"),
  country("JP", "Japan", [2, "moderate"]),
  country("CL", "Chile", [1, "extreme"]),
  country("ID", "Indonesia", [5, "moderate"]),
  country("AF", "Afghanistan"),
  country("CI", "Côte d'Ivoire"),
];

function summary(active: number, events: HazardEvent[], counts: CountrySummary["counts"]): CountrySummary {
  return {
    country: { ...country("CH", "Switzerland"), activeEvents: active },
    counts,
    coverage: { officialWarnings: true, sources: ["meteoalarm"] },
    events,
    recentChanges: [],
    generatedAt: "2027-01-15T08:00:00Z",
  };
}

describe(directory, () => {
  it("puts active countries first, most severe then busiest, then the rest by name", () => {
    const listed = directory(ALL, "");
    expect(listed.active.map((each) => each.iso2)).toStrictEqual(["CL", "ID", "JP"]);
    expect(listed.quiet.map((each) => each.iso2)).toStrictEqual(["AF", "CI", "PT"]);
  });

  it("keeps every match of a filter, not just five", () => {
    const listed = directory(ALL, "a");
    expect(listed.active.map((each) => each.iso2)).toStrictEqual(["ID", "JP"]);
    expect(listed.quiet.map((each) => each.iso2)).toStrictEqual(["AF", "PT"]);
    expect(directory(ALL, "cote").quiet.map((each) => each.iso2)).toStrictEqual(["CI"]);
    expect(directory(ALL, "JPX").active.map((each) => each.iso2)).toStrictEqual(["JP"]);
  });

  it("is empty when nothing matches", () => {
    const listed = directory(ALL, "zz");
    expect([...listed.active, ...listed.quiet]).toStrictEqual([]);
  });
});

describe(countryCells, () => {
  it("counts warnings from the list, as a lower bound when the list is capped", () => {
    const events = [makeEvent({ kind: "warning", severity: "severe" }), makeEvent({ kind: "observed" })];
    const counts = [{ type: "flood", severity: "severe" as const, count: 150 }];
    expect(countryCells(summary(150, events, counts)).map((cell) => cell.value)).toStrictEqual(["150", "150", "1+"]);
    expect(countryCells(summary(2, events, counts)).map((cell) => cell.value)).toStrictEqual(["2", "150", "1"]);
  });
});

describe(capNote, () => {
  const counts = [{ type: "flood", severity: "moderate" as const, count: 120 }];
  const listed = Array.from({ length: 100 }, () => makeEvent({ type: "flood", severity: "moderate" }));

  it("says nothing when the list is whole", () => {
    expect(capNote(summary(100, listed, counts), 100, undefined)).toBeUndefined();
  });

  it("says how many of how many a capped list holds", () => {
    expect(capNote(summary(120, listed, counts), 100, undefined)).toBe("Showing the 100 most severe of 120 events.");
    expect(capNote(summary(120, listed, counts), 100, { type: "flood", severity: "moderate" })).toBe(
      "Showing 100 of 120. The list holds the 100 most severe events in Switzerland.",
    );
  });
});

describe(activeWords, () => {
  it("names the count and the worst severity, in or near", () => {
    expect(activeWords(country("CL", "Chile", [1, "extreme"]))).toBe(
      "Chile, 1 active event in or near it, up to extreme",
    );
    expect(activeWords(country("PT", "Portugal"))).toBe("Portugal, no active events");
    expect(cellWords({ type: "wildfire", severity: "severe" })).toBe("Wildfire · Severe");
  });
});

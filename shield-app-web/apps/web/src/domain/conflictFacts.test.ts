import { describe, expect, it } from "vite-plus/test";

import { conflictDateLine, detailTimes, isApproximateLocation, keyFigures, locationNote } from "./eventFacts.ts";
import { makeEvent } from "./testEvents.ts";

/** shield-core ConflictTests (ADR 27, docs/attrs.md "UCDP"). */

const now = new Date("2026-11-19T17:40:00Z");

/** The live Yemen row on 2026-11-19, the largest in the 90 days. */
const yemen = (precision = "area", codeStatus = "Check deaths"): ReturnType<typeof makeEvent> =>
  makeEvent({
    source: "ucdp",
    type: "conflict",
    severity: "extreme",
    sourceSeverity: "797 deaths (335–823)",
    severityProvisional: true,
    title: "Government of Yemen (North Yemen) – Forces of the Presidential Leadership Council",
    countries: [{ iso2: "YE", relation: "within" }],
    startedAt: "2026-08-09T00:00:00Z",
    observedAt: "2026-09-10T00:00:00Z",
    updatedAt: "2026-09-10T00:00:00Z",
    attrs: {
      violenceType: "state_based",
      conflictName: "Yemen (North Yemen): Government",
      sideA: "Government of Yemen (North Yemen)",
      sideB: "Forces of the Presidential Leadership Council",
      deathsBest: 797,
      deathsLow: 335,
      deathsHigh: 823,
      deathsSideA: 456,
      deathsSideB: 341,
      deathsCivilians: 0,
      deathsUnknown: 0,
      locationPrecision: precision,
      datePrecision: "months",
      codeStatus,
      sourceCount: 2,
      expiresAt: "2026-12-09T00:00:00Z",
    },
  });

const oneSided = (): ReturnType<typeof makeEvent> => {
  const base = yemen("exact", "Clear");
  return {
    ...base,
    startedAt: "2026-09-21T00:00:00Z",
    observedAt: "2026-09-21T00:00:00Z",
    attrs: {
      ...base.attrs,
      violenceType: "one_sided",
      sideA: "IS",
      sideB: "Civilians",
      deathsSideA: 0,
      deathsSideB: 0,
      deathsCivilians: 12,
    },
  };
};

const rows = (event: ReturnType<typeof makeEvent>): string[] =>
  keyFigures(event).map((figure) => `${figure.label}: ${figure.value}`);

describe("conflict key figures", () => {
  it("say who died and how sure the estimate is", () => {
    expect(rows(yemen())).toStrictEqual([
      "Deaths: 797",
      "Estimate range: 335–823",
      "Violence: State-based (a government is a side)",
      "Conflict: Yemen (North Yemen): Government",
      "Side A: Government of Yemen (North Yemen) · 456 deaths",
      "Side B: Forces of the Presidential Leadership Council · 341 deaths",
      "Location: Wider area (approximate)",
      "UCDP review: Check deaths",
      "Sources used: 2",
    ]);
    expect(keyFigures(yemen())[0]?.isLead).toBe(true);
  });

  it("count the civilians for one-sided violence", () => {
    const figures = rows(oneSided());
    expect(figures).toContain("Violence: One-sided (against civilians)");
    expect(figures).toContain("Side B: Civilians · 12 deaths");
    expect(figures.map((row) => row.split(":")[0])).not.toContain("Location");
    expect(figures.map((row) => row.split(":")[0])).not.toContain("UCDP review");
  });
});

describe("conflict location", () => {
  it.each([
    ["admin2", "Approximate location: UCDP places this at the district’s centre."],
    ["admin1", "Approximate location: UCDP places this at the province’s centre."],
    ["within_25km", "Approximate location: within 25 km of the point shown."],
    ["area", "Approximate location: UCDP records a wider area or a line, shown at its centre."],
  ])("says %s is approximate", (precision, expected) => {
    expect(locationNote(yemen(precision))).toBe(expected);
  });

  it("says nothing for an exact one", () => {
    expect(locationNote(yemen("exact"))).toBeUndefined();
    expect(isApproximateLocation(yemen())).toBe(true);
    expect(isApproximateLocation(oneSided())).toBe(false);
  });
});

describe("conflict dates", () => {
  it("are days, never updates", () => {
    // UCDP gives no edit time: the event's own dates, never "updated N ago".
    const times = detailTimes(yemen(), now);
    expect(times.map((time) => time.label)).toStrictEqual(["From", "To", "Shown until"]);
    expect(times.every((time) => time.dateOnly)).toBe(true);
    expect(detailTimes(oneSided(), now).map((time) => time.label)).toStrictEqual(["Date", "Shown until"]);
  });

  it("read as a monthly estimate on the row", () => {
    expect(conflictDateLine(yemen())).toBe("9 Aug – 10 Sep · monthly estimate (UCDP)");
    expect(conflictDateLine(oneSided())).toBe("21 Sep · monthly estimate (UCDP)");
  });
});

import { describe, expect, it } from "vite-plus/test";

import {
  arrivalLine,
  detailTimes,
  formatNumber,
  isPrescribed,
  keyFigures,
  severityNote,
  severityRating,
  showsProvisional,
} from "./eventFacts.ts";
import { AT, makeEvent } from "./testEvents.ts";

const hours = (count: number): Date => new Date(AT.getTime() + count * 3_600_000);
const rows = (event: Parameters<typeof keyFigures>[0]): string[] =>
  keyFigures(event).map((figure) => `${figure.label}: ${figure.value}`);

describe(severityRating, () => {
  it("names the wind speed that decided", () => {
    const event = makeEvent({
      sourceSeverity: "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 194 km/h)",
      attrs: { severityBasis: "wind", severityValue: 240.7392, severityUnit: "km/h", alertLevel: "Green" },
    });
    expect(severityRating(event)).toBe("Rated by wind speed · 241 km/h");
    expect(severityNote(event)).toBe("GDACS’s own alert is Green.");
  });

  it("names the burned area that decided", () => {
    const event = makeEvent({
      type: "wildfire",
      severity: "extreme",
      attrs: { severityBasis: "area", severityValue: 48_484, severityUnit: "ha", alertLevel: "Green" },
    });
    expect(severityRating(event)).toBe("Rated by burned area · 48,484 ha");
    expect(severityNote(event)).toBe("GDACS’s own alert is Green.");
  });

  it("says nothing when GDACS's own alert decided", () => {
    const event = makeEvent({ attrs: { severityBasis: "alert", alertLevel: "Orange" } });
    expect(severityRating(event)).toBeUndefined();
    expect(severityNote(event)).toBeUndefined();
  });
});

describe(severityNote, () => {
  it("lets a prescribed burn win over the provisional words", () => {
    const event = makeEvent({
      source: "eonet",
      type: "wildfire",
      severity: "minor",
      sourceSeverity: "1,402 acres burned",
      severityProvisional: true,
      attrs: { prescribed: true, magnitudeValue: 1401.8, magnitudeUnit: "acres" },
    });
    expect(isPrescribed(event)).toBe(true);
    expect(severityRating(event)).toBe("Prescribed burn");
    expect(severityNote(event)).toBe("A planned, controlled fire, shown as minor whatever its size.");
    expect(showsProvisional(event)).toBe(false);
  });

  it("follows DESIGN.md §5 for provisional severities", () => {
    const fire = makeEvent({
      source: "eonet",
      type: "wildfire",
      severity: "moderate",
      severityProvisional: true,
      attrs: { magnitudeValue: 2378.77, magnitudeUnit: "acres" },
    });
    expect(severityRating(fire)).toBeUndefined();
    expect(showsProvisional(fire)).toBe(true);
    expect(severityNote(fire)).toBe("Estimated from burned area; the source publishes no severity.");
    const volcano = makeEvent({ source: "eonet", type: "volcano", severity: "minor", severityProvisional: true });
    expect(severityNote(volcano)).toBe(
      "The source publishes no scale; shown as minor until an official source rates it.",
    );
  });

  it("reads the provisional flag older payloads kept in attrs", () => {
    expect(showsProvisional(makeEvent({ attrs: { severityProvisional: true } }))).toBe(true);
    expect(showsProvisional(makeEvent())).toBe(false);
  });
});

describe(keyFigures, () => {
  it("leads an earthquake with its magnitude", () => {
    const event = makeEvent({
      source: "usgs",
      type: "earthquake",
      attrs: { mag: 6.1, magType: "mww", depthKm: 10, pagerAlert: "green", tsunami: true, reviewStatus: "reviewed" },
    });
    expect(rows(event)).toStrictEqual([
      "Magnitude: 6.1 mww",
      "Depth: 10 km",
      "PAGER: Green",
      "Tsunami: Possible (USGS flag)",
      "Review: Reviewed",
    ]);
    expect(keyFigures(event)[0]?.isLead).toBe(true);
  });

  it("shows a cyclone's wind in the source's unit", () => {
    const event = makeEvent({
      attrs: {
        severityValue: 83.3328,
        severityUnit: "km/h",
        vulnerability: "High",
        populationText: "28 million in Tropical Storm",
      },
    });
    expect(rows(event)).toStrictEqual([
      "Max wind: 83 km/h",
      "Population: 28 million in Tropical Storm",
      "Vulnerability: High",
    ]);
  });

  it("shows a wildfire's burned area", () => {
    const eonet = { source: "eonet", type: "wildfire" };
    const acres = makeEvent({ ...eonet, attrs: { magnitudeValue: 2378.77, magnitudeUnit: "acres" } });
    expect(keyFigures(acres).map((figure) => figure.value)).toStrictEqual(["2,379 acres"]);
    const hectare = makeEvent({ ...eonet, attrs: { magnitudeValue: 5000, magnitudeUnit: "hectare" } });
    expect(keyFigures(hectare).map((figure) => figure.value)).toStrictEqual(["5,000 ha"]);
    const gdacs = makeEvent({ type: "wildfire", attrs: { severityValue: 48_484, severityUnit: "ha" } });
    expect(keyFigures(gdacs)[0]).toStrictEqual({ label: "Burned area", value: "48,484 ha", isLead: true });
  });

  it("leaves out a value without a known meaning", () => {
    expect(keyFigures(makeEvent({ type: "flood", attrs: { severityValue: 0, severityUnit: "" } }))).toStrictEqual([]);
    expect(keyFigures(makeEvent({ attrs: { severityValue: 120 } }))).toStrictEqual([]);
  });

  it("shows a warning's CAP axes apart", () => {
    const event = makeEvent({
      source: "meteoalarm",
      type: "wind",
      kind: "warning",
      attrs: {
        urgency: "Immediate",
        certainty: "Likely",
        sender: "Deutscher Wetterdienst",
        areas: ["Kreis Harz", "Goslar"],
      },
    });
    expect(keyFigures(event).map((figure) => figure.label)).toStrictEqual([
      "Urgency",
      "Certainty",
      "Issued by",
      "Areas",
    ]);
    expect(keyFigures(event).at(-1)?.value).toBe("Kreis Harz, Goslar");
  });
});

describe(detailTimes, () => {
  it("ends a warning when it expires, and an ended event when it ended", () => {
    const warning = makeEvent({
      source: "meteoalarm",
      kind: "warning",
      startedAt: hours(-1).toISOString(),
      attrs: { expiresAt: "2026-11-16T18:00:00Z" },
    });
    expect(detailTimes(warning, AT).map((time) => time.label)).toStrictEqual([
      "Started",
      "Observed",
      "Updated",
      "Expires",
    ]);
    expect(detailTimes(warning, AT).at(-1)?.date.getTime()).toBe(1_794_852_000_000);
    const ended = makeEvent({ endedAt: AT.toISOString() });
    expect(detailTimes(ended, AT).map((time) => time.label)).toStrictEqual(["Observed", "Updated", "Ended"]);
  });

  it("says Starts for a warning that has not begun, never Started in", () => {
    const event = makeEvent({ source: "meteoalarm", kind: "warning", startedAt: hours(7).toISOString() });
    expect(detailTimes(event, AT)[0]?.label).toBe("Starts");
    expect(detailTimes(event, hours(8))[0]?.label).toBe("Started");
  });

  it("leaves out an observation equal to the onset", () => {
    const event = makeEvent({ startedAt: AT.toISOString() });
    expect(detailTimes(event, AT).map((time) => time.label)).toStrictEqual(["Started", "Updated"]);
  });
});

describe(arrivalLine, () => {
  const quake = makeEvent({
    type: "earthquake",
    attrs: { mag: 6.1, magType: "mww" },
    countries: [{ iso2: "CL", relation: "within" }],
  });

  it("names the severity, the type, the magnitude and the first country", () => {
    expect(arrivalLine(quake)).toBe("New · Severe earthquake · M6.1 · Chile");
  });

  it("uses the lead figure without a magnitude", () => {
    const fire = {
      ...quake,
      type: "wildfire",
      severity: "extreme" as const,
      attrs: { magnitudeValue: 48_484, magnitudeUnit: "hectares" },
      countries: [],
    };
    expect(arrivalLine(fire)).toBe("New · Extreme wildfire · 48,484 ha");
  });
});

describe("magnitude words", () => {
  it("round a tie to even, as the apps' %.1f does", () => {
    const quake = makeEvent({ type: "earthquake", attrs: { mag: 4.25 } });
    expect(arrivalLine(quake)).toBe("New · Severe earthquake · M4.2");
    expect(keyFigures(quake)[0]?.value).toBe("4.2");
  });
});

describe(formatNumber, () => {
  it("groups whole numbers in en-US", () => {
    expect(formatNumber(48_484)).toBe("48,484");
    expect(formatNumber(240.7392)).toBe("241");
  });
});

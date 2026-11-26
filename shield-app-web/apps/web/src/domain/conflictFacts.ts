import type { HazardEvent } from "@shield/api-client";

import type { Attrs, DetailTime, FigureRow, KeyFigure } from "./attrs.ts";
import { attrDate, attrInt, attrString, figures, formatNumber } from "./attrs.ts";
import { utcDay } from "./freshness.ts";
import { CONFLICT } from "./taxonomy.ts";

/** Armed conflicts (UCDP, ADR 27; docs/attrs.md "UCDP"). */

const VIOLENCE: ReadonlyMap<string, string> = new Map([
  ["state_based", "State-based (a government is a side)"],
  ["non_state", "Non-state (organized groups)"],
  ["one_sided", "One-sided (against civilians)"],
]);

const LOCATION: ReadonlyMap<string, string> = new Map([
  ["within_25km", "Within 25 km"],
  ["admin2", "District (approximate)"],
  ["admin1", "Province (approximate)"],
  ["area", "Wider area (approximate)"],
]);

const LOCATION_NOTES: ReadonlyMap<string, string> = new Map([
  ["within_25km", "Approximate location: within 25 km of the point shown."],
  ["admin2", "Approximate location: UCDP places this at the district’s centre."],
  ["admin1", "Approximate location: UCDP places this at the province’s centre."],
  ["area", "Approximate location: UCDP records a wider area or a line, shown at its centre."],
]);

const CIVILIANS = "Civilians";

/** "1 death", "797 deaths". */
function deaths(count: number): string {
  return count === 1 ? "1 death" : `${formatNumber(count)} deaths`;
}

/** Anything but an exact geocode is drawn as approximate. */
function isApproximateLocation(event: Pick<HazardEvent, "type" | "attrs">): boolean {
  const precision = attrString(event.attrs, "locationPrecision");
  return event.type === CONFLICT && precision !== undefined && precision !== "exact";
}

/** What the detail says about a rough geocode; undefined for an exact one. */
function conflictLocationNote(attrs: Attrs): string | undefined {
  const precision = attrString(attrs, "locationPrecision");
  return precision === undefined ? undefined : LOCATION_NOTES.get(precision);
}

/** The estimate's range, when it is one: "335–823". */
function rangeRow(attrs: Attrs): FigureRow {
  const low = attrInt(attrs, "deathsLow");
  const high = attrInt(attrs, "deathsHigh");
  if (low === undefined || high === undefined || low === high) {
    return ["Estimate range", undefined];
  }
  return ["Estimate range", `${formatNumber(low)}–${formatNumber(high)}`];
}

/** Each side with its dead; one-sided violence counts the civilians on side B. */
function sideRows(attrs: Attrs): FigureRow[] {
  const civilians = attrInt(attrs, "deathsCivilians") ?? 0;
  const sideA = attrString(attrs, "sideA");
  const sideB = attrString(attrs, "sideB");
  const sideBDeaths = sideB === CIVILIANS ? civilians : (attrInt(attrs, "deathsSideB") ?? 0);
  const unknown = attrInt(attrs, "deathsUnknown") ?? 0;
  return [
    ["Side A", sideA === undefined ? undefined : `${sideA} · ${deaths(attrInt(attrs, "deathsSideA") ?? 0)}`],
    ["Side B", sideB === undefined ? undefined : `${sideB} · ${deaths(sideBDeaths)}`],
    ["Civilians", sideB !== CIVILIANS && civilians > 0 ? deaths(civilians) : undefined],
    ["Unknown", unknown > 0 ? deaths(unknown) : undefined],
  ];
}

/** Who died and how sure UCDP is, deaths leading. */
function conflictFigures(attrs: Attrs): KeyFigure[] {
  const best = attrInt(attrs, "deathsBest");
  const violence = attrString(attrs, "violenceType");
  const precision = attrString(attrs, "locationPrecision");
  const status = attrString(attrs, "codeStatus");
  const sources = attrInt(attrs, "sourceCount");
  return figures([
    ["Deaths", best === undefined ? undefined : formatNumber(best), true],
    rangeRow(attrs),
    ["Violence", violence === undefined ? undefined : VIOLENCE.get(violence)],
    ["Conflict", attrString(attrs, "conflictName")],
    ...sideRows(attrs),
    ["Location", precision === undefined ? undefined : LOCATION.get(precision)],
    ["UCDP review", status === "Clear" ? undefined : status],
    ["Sources used", sources === undefined ? undefined : String(sources)],
  ]);
}

/** True when two dates fall on the same UTC day. */
function sameUtcDay(left: Date, right: Date): boolean {
  return utcDay(left) === utcDay(right);
}

/** From and To (or one Date), then when the app stops showing it: days, not times. */
function conflictTimes(event: Pick<HazardEvent, "startedAt" | "observedAt" | "attrs">): DetailTime[] {
  const observed = new Date(event.observedAt);
  const from = event.startedAt === undefined ? observed : new Date(event.startedAt);
  const days: DetailTime[] = sameUtcDay(from, observed)
    ? [{ label: "Date", date: observed, dateOnly: true }]
    : [
        { label: "From", date: from, dateOnly: true },
        { label: "To", date: observed, dateOnly: true },
      ];
  const until = attrDate(event.attrs, "expiresAt");
  return until === undefined ? days : [...days, { label: "Shown until", date: until, dateOnly: true }];
}

/** The row's line, in place of "updated N ago": "9 Aug – 10 Sep · monthly estimate (UCDP)". */
function conflictDateLine(event: Pick<HazardEvent, "startedAt" | "observedAt">): string {
  const observed = new Date(event.observedAt);
  const from = event.startedAt === undefined ? observed : new Date(event.startedAt);
  const days = sameUtcDay(from, observed) ? utcDay(observed) : `${utcDay(from)} – ${utcDay(observed)}`;
  return `${days} · monthly estimate (UCDP)`;
}

export { conflictDateLine, conflictFigures, conflictLocationNote, conflictTimes, isApproximateLocation };

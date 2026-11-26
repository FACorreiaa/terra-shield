import type { HazardEvent } from "@shield/api-client";

import type { Attrs, DetailTime, FigureRow, KeyFigure } from "./attrs.ts";
import {
  attrArray,
  attrBool,
  attrDate,
  attrInt,
  attrNumber,
  attrString,
  attrStrings,
  capitalizeWords,
  figures,
  formatMagnitude,
  formatNumber,
} from "./attrs.ts";
import { conflictFigures, conflictLocationNote, conflictTimes } from "./conflictFacts.ts";
import { countryName } from "./place.ts";
import { CONFLICT, severityLabel, typeLabel } from "./taxonomy.ts";
import { topBand, tsunamiFigures } from "./tsunamiFacts.ts";

/**
 * What the event detail (DESIGN.md §10.3) says beyond the badge: why the severity, the key figures
 * and the times. Only what the source sent; nothing derived it didn't say (shield-core EventFacts.swift).
 */

/** What decided a GDACS severity (`attrs.severityBasis`, docs/severity.md); `alert` needs no words. */
const RATING_LABELS: ReadonlyMap<string, string> = new Map([
  ["wind", "Rated by wind speed"],
  ["area", "Rated by burned area"],
]);

/** GDACS's structured value has a nameable meaning for these types only (floods send "Magnitude 0"). */
const GDACS_VALUE_LABELS: ReadonlyMap<string, string> = new Map([
  ["tropical_cyclone", "Max wind"],
  ["wildfire", "Burned area"],
]);

const HECTARE_UNITS: ReadonlySet<string> = new Set(["hectare", "hectares"]);
const GUIDANCE = "Guidance for national authorities, not a warning.";
const HAZARDOUS_WAVES = "hazardous waves possible";
const EPICENTRE_NOTE = "Shown at the earthquake’s epicentre. The waves reach the places listed below.";

type Facts = Pick<HazardEvent, "type" | "kind" | "severity" | "severityProvisional" | "attrs">;

/** An EONET prescribed burn (`attrs.prescribed`): a planned fire, always minor. */
function isPrescribed(event: Pick<HazardEvent, "attrs">): boolean {
  return attrBool(event.attrs, "prescribed") === true;
}

/** The top-level flag, or the attrs copy older payloads carried. */
function isProvisional(event: Pick<HazardEvent, "severityProvisional" | "attrs">): boolean {
  return event.severityProvisional ?? attrBool(event.attrs, "severityProvisional") ?? false;
}

/** Whether the badge says "Provisional". A prescribed burn is minor by rule, not by estimate. */
function showsProvisional(event: Pick<HazardEvent, "severityProvisional" | "attrs">): boolean {
  return isProvisional(event) && !isPrescribed(event);
}

/** GDACS's structured value with its unit ("241 km/h"); undefined without a positive value and a unit. */
function gdacsValue(attrs: Attrs): string | undefined {
  const value = attrNumber(attrs, "severityValue");
  const unit = attrString(attrs, "severityUnit");
  if (value === undefined || value <= 0 || unit === undefined || unit === "") {
    return undefined;
  }
  return `${formatNumber(value)} ${unit}`;
}

/**
 * "Rated by wind speed · 241 km/h", "Rated by burned area · 48,484 ha", "Prescribed burn" (owner,
 * 2026-11-19); undefined when the source's own scale decided.
 */
function severityRating(event: Pick<HazardEvent, "attrs">): string | undefined {
  if (isPrescribed(event)) {
    return "Prescribed burn";
  }
  const basis = attrString(event.attrs, "severityBasis");
  const label = basis === undefined ? undefined : RATING_LABELS.get(basis);
  if (label === undefined) {
    return undefined;
  }
  const value = gdacsValue(event.attrs);
  return value === undefined ? label : `${label} · ${value}`;
}

/** PTWC threat forecasts (#25): advice to governments, never a warning to the public. */
function guidanceNote(event: Facts): string {
  if (!isProvisional(event) || topBand(event.attrs) !== HAZARDOUS_WAVES) {
    return GUIDANCE;
  }
  return `${GUIDANCE} Rated ${severityLabel(event.severity).toLowerCase()} until wave heights are forecast.`;
}

/** One line on why the severity reads as it does, or undefined when the badge says it all. */
function severityNote(event: Facts): string | undefined {
  if (isPrescribed(event)) {
    return "A planned, controlled fire, shown as minor whatever its size.";
  }
  if (attrBool(event.attrs, "guidance") === true) {
    return guidanceNote(event);
  }
  const basis = attrString(event.attrs, "severityBasis");
  if (basis !== undefined && RATING_LABELS.has(basis)) {
    const alert = attrString(event.attrs, "alertLevel");
    return alert === undefined || alert === "" ? undefined : `GDACS’s own alert is ${alert}.`;
  }
  if (!isProvisional(event)) {
    return undefined;
  }
  // DESIGN.md §5.
  if (attrNumber(event.attrs, "magnitudeValue") !== undefined) {
    return "Estimated from burned area; the source publishes no severity.";
  }
  return "The source publishes no scale; shown as minor until an official source rates it.";
}

/** USGS: magnitude leads, then depth, PAGER, the tsunami flag, felt reports and review. */
function earthquakeFigures(attrs: Attrs): KeyFigure[] {
  const magnitude = attrNumber(attrs, "mag");
  const magnitudeType = attrString(attrs, "magType");
  const depth = attrNumber(attrs, "depthKm");
  const pager = attrString(attrs, "pagerAlert");
  const felt = attrInt(attrs, "feltReports");
  const review = attrString(attrs, "reviewStatus");
  const magnitudeWords =
    magnitude === undefined
      ? undefined
      : [formatMagnitude(magnitude), magnitudeType].filter((part) => part !== undefined).join(" ");
  return figures([
    ["Magnitude", magnitudeWords, true],
    ["Depth", depth === undefined ? undefined : `${formatNumber(depth)} km`],
    ["PAGER", pager === undefined ? undefined : capitalizeWords(pager)],
    ["Tsunami", attrBool(attrs, "tsunami") === true ? "Possible (USGS flag)" : undefined],
    ["Felt reports", felt === undefined ? undefined : formatNumber(felt)],
    ["Review", review === undefined ? undefined : capitalizeWords(review)],
  ]);
}

/** GDACS's value for a cyclone or a fire, then EONET's latest burned area. */
function magnitudeRows(type: string, attrs: Attrs): FigureRow[] {
  const gdacsLabel = GDACS_VALUE_LABELS.get(type);
  const gdacs: FigureRow[] = gdacsLabel === undefined ? [] : [[gdacsLabel, gdacsValue(attrs), true]];
  const value = attrNumber(attrs, "magnitudeValue");
  if (type !== "wildfire" || value === undefined || value <= 0) {
    return gdacs;
  }
  const rawUnit = attrString(attrs, "magnitudeUnit") ?? "";
  const unit = HECTARE_UNITS.has(rawUnit) ? "ha" : rawUnit;
  const area = [formatNumber(value), unit].filter((part) => part !== "").join(" ");
  return [...gdacs, ["Burned area", area, true]];
}

/** GDACS, EONET and CAP figures. CAP's axes are each their own field, never folded into severity. */
function otherFigures(type: string, attrs: Attrs): KeyFigure[] {
  const areas = attrArray(attrs, "areas") === undefined ? undefined : attrStrings(attrs, "areas").join(", ");
  return figures([
    ...magnitudeRows(type, attrs),
    ["Population", attrString(attrs, "populationText")],
    ["Vulnerability", attrString(attrs, "vulnerability")],
    ["Urgency", attrString(attrs, "urgency")],
    ["Certainty", attrString(attrs, "certainty")],
    ["Issued by", attrString(attrs, "sender")],
    ["Areas", areas],
  ]);
}

/** The figures `attrs` holds for this type and source, in reading order. Only what exists. */
function keyFigures(event: Pick<HazardEvent, "type" | "attrs">): KeyFigure[] {
  if (event.type === "tsunami" && event.attrs["center"] !== undefined) {
    return tsunamiFigures(event.attrs);
  }
  if (event.type === CONFLICT) {
    return conflictFigures(event.attrs);
  }
  if (event.type === "earthquake") {
    return earthquakeFigures(event.attrs);
  }
  return otherFigures(event.type, event.attrs);
}

type Timed = Pick<HazardEvent, "type" | "kind" | "startedAt" | "observedAt" | "updatedAt" | "endedAt" | "attrs">;

/** Ended, else the expiry: a PTWC forecast's expiresAt is Shield's 12 h net, not the centre's (#25). */
function closingTime(event: Timed): DetailTime[] {
  if (event.endedAt !== undefined) {
    return [{ label: "Ended", date: new Date(event.endedAt), dateOnly: false }];
  }
  const expires = attrDate(event.attrs, "expiresAt");
  if (expires === undefined) {
    return [];
  }
  const isNet = event.type === "tsunami" && event.kind === "forecast";
  return [{ label: isNet ? "Shown until" : "Expires", date: expires, dateOnly: false }];
}

/**
 * Started (or "Starts", for a warning ahead of its onset), observed, updated, then ended or expires.
 * A time equal to the one before it is left out (MeteoAlarm's onset is also its observation time).
 */
function detailTimes(event: Timed, now: Date = new Date()): DetailTime[] {
  if (event.type === CONFLICT) {
    return conflictTimes(event);
  }
  const started = event.startedAt === undefined ? undefined : new Date(event.startedAt);
  const observed = new Date(event.observedAt);
  const opening: DetailTime[] = [];
  if (started !== undefined) {
    const label = started.getTime() > now.getTime() ? "Starts" : "Started";
    opening.push({ label, date: started, dateOnly: false });
  }
  if (started?.getTime() !== observed.getTime()) {
    opening.push({ label: "Observed", date: observed, dateOnly: false });
  }
  return [...opening, { label: "Updated", date: new Date(event.updatedAt), dateOnly: false }, ...closingTime(event)];
}

/**
 * Where the marker is not: a tsunami forecast sits on the earthquake (`attrs.geometry == "epicenter"`),
 * and a conflict geocoded only roughly says so. Undefined when the map draws the place itself.
 */
function locationNote(event: Pick<HazardEvent, "type" | "attrs">): string | undefined {
  if (event.type === CONFLICT) {
    return conflictLocationNote(event.attrs);
  }
  if (event.type !== "tsunami" || attrString(event.attrs, "geometry") !== "epicenter") {
    return undefined;
  }
  return EPICENTRE_NOTE;
}

/** A live arrival (DESIGN.md §10.1): "New · Severe earthquake · M6.1 · Chile". */
function arrivalLine(event: Pick<HazardEvent, "type" | "severity" | "attrs" | "countries">): string {
  const magnitude = event.type === "earthquake" ? attrNumber(event.attrs, "mag") : undefined;
  const figure =
    magnitude === undefined
      ? keyFigures(event).find((candidate) => candidate.isLead)?.value
      : `M${formatMagnitude(magnitude)}`;
  const [country] = event.countries;
  return [
    "New",
    `${severityLabel(event.severity)} ${typeLabel(event.type).toLowerCase()}`,
    figure,
    country === undefined ? undefined : countryName(country.iso2),
  ]
    .filter((part) => part !== undefined)
    .join(" · ");
}

export type { DetailTime, KeyFigure } from "./attrs.ts";
export { formatNumber } from "./attrs.ts";
export { conflictDateLine, isApproximateLocation } from "./conflictFacts.ts";
export {
  arrivalLine,
  detailTimes,
  isPrescribed,
  keyFigures,
  locationNote,
  severityNote,
  severityRating,
  showsProvisional,
};

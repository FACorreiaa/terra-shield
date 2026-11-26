/**
 * Countries: the search field, where the map goes, and a country's summary (DESIGN.md §10.2).
 * Mirrors shield-core `Country` and `CountrySummary` (Countries.swift).
 */
import type { components, HazardEvent, Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";

import type { GeoBox } from "./geo.ts";
import { geoBox, longitudeSpan, WORLD } from "./geo.ts";
import type { EventType } from "./taxonomy.ts";
import { compareSeverity, eventType, isAtLeast, typeLabel } from "./taxonomy.ts";

type Country = components["schemas"]["Country"];
type CountrySummary = components["schemas"]["CountrySummary"];

/** A row of the type × severity matrix. */
interface MatrixRow {
  type: EventType;
  counts: Partial<Record<Severity, number>>;
  total: number;
  /** The highest severity with events; minor for an empty row. */
  maxSeverity: Severity;
}

/** One cell of the matrix: choosing it filters the country's list. */
interface MatrixCell {
  type: EventType;
  severity: Severity;
}

/** The server lists at most this many events in a summary (`store/country_stats.go`). */
const EVENT_LIMIT = 100;
/** Search lists this many countries. */
const MATCH_LIMIT = 5;
/** An ISO code is 2 letters, or 3. */
const LONGEST_CODE = 3;
/** A bbox wider than this crosses ±180° (Russia, the US, Fiji…) and would frame the whole world. */
const WRAPPED_SPAN = 300;
const HALF_TURN = 180;
const FULL_TURN = 360;
const ISO2 = /^[A-Z]{2}$/u;
const DIACRITICS = /\p{Diacritic}/gu;

/** "PT" for "pt" or " Pt ", undefined for anything that isn't two letters A–Z (the API answers 400). */
function normalizedCode(raw: string): string | undefined {
  const code = raw.trim().toUpperCase();
  return ISO2.test(code) ? code : undefined;
}

/** Lower case without accents, so "Côte" matches "cote". */
function fold(text: string): string {
  return text.normalize("NFD").replaceAll(DIACRITICS, "").toLowerCase();
}

/** 0 for an exact ISO code, 1 for a name that starts with the text, 2 for one that contains it. */
function rank(country: Country, query: string, folded: string): number | undefined {
  if (folded.length <= LONGEST_CODE) {
    const code = query.toUpperCase();
    if (country.iso2 === code || country.iso3 === code) {
      return 0;
    }
  }
  const name = fold(country.name);
  if (name.startsWith(folded)) {
    return 1;
  }
  return name.includes(folded) ? 2 : undefined;
}

/**
 * Countries for the search field: an exact ISO code first (2 or 3 letters), then names that start
 * with the text, then names that contain it. Case and accents are ignored.
 */
function matching(text: string, countries: readonly Country[], limit: number = MATCH_LIMIT): Country[] {
  const query = text.trim();
  if (query === "") {
    return [];
  }
  const folded = fold(query);
  return countries
    .flatMap((country) => {
      const position = rank(country, query, folded);
      return position === undefined ? [] : [{ country, position }];
    })
    .toSorted(
      (left, right) =>
        left.position - right.position ||
        left.country.name.localeCompare(right.country.name, "en", { numeric: true, sensitivity: "accent" }),
    )
    .slice(0, limit)
    .map((ranked) => ranked.country);
}

/**
 * The shortest stretch of longitude that holds every value: the complement of the widest gap
 * between neighbours on the circle. `west > east` when it crosses ±180°.
 */
function smallestArc(longitudes: readonly number[]): [west: number, east: number] {
  const sorted = longitudes.toSorted((left, right) => left - right);
  const first = sorted.at(0);
  const last = sorted.at(-1);
  if (first === undefined || last === undefined) {
    return [-HALF_TURN, HALF_TURN];
  }
  // The gap that wraps from the last value round to the first.
  let widest = { gap: first + FULL_TURN - last, west: first, east: last };
  for (const [index, east] of sorted.slice(0, -1).entries()) {
    const west = sorted[index + 1] ?? east;
    if (west - east > widest.gap) {
      widest = { gap: west - east, west, east };
    }
  }
  return [widest.west, widest.east];
}

/**
 * Where the map goes for a country: usually its bbox. A bbox nearly 360° wide (the country crosses
 * ±180°) would frame the whole world, so then it is the smallest box around `events`, which may
 * cross ±180°, or, with no events, the country's band of latitude around the world.
 */
function mapBox(country: Pick<Country, "bbox">, events: readonly Pick<HazardEvent, "centroid">[]): GeoBox {
  const box = geoBox(country.bbox) ?? WORLD;
  if (longitudeSpan(box) <= WRAPPED_SPAN) {
    return box;
  }
  if (events.length === 0) {
    return { west: -HALF_TURN, south: box.south, east: HALF_TURN, north: box.north };
  }
  const latitudes = events.map((event) => event.centroid.latitude);
  const [west, east] = smallestArc(events.map((event) => event.centroid.longitude));
  return { west, south: Math.min(...latitudes), east, north: Math.max(...latitudes) };
}

/** Fewer events listed than are active: the list holds the most severe ones. */
function isListCapped(summary: Pick<CountrySummary, "country" | "events">): boolean {
  return summary.events.length < summary.country.activeEvents;
}

/** Severe and extreme, from the counts, so exact even when the list is capped. */
function highSeverityCount(summary: Pick<CountrySummary, "counts">): number {
  return summary.counts.filter((cell) => isAtLeast(cell.severity, "severe")).reduce((sum, cell) => sum + cell.count, 0);
}

/**
 * Official warnings. The counts carry no kind, so this comes from the listed events: at least
 * this many when the list is capped.
 */
function warningCount(summary: Pick<CountrySummary, "country" | "events">): { value: number; isLowerBound: boolean } {
  const value = summary.events.filter((event) => event.kind === "warning").length;
  return { value, isLowerBound: isListCapped(summary) };
}

function row(type: EventType, counts: Partial<Record<Severity, number>>): MatrixRow {
  const total = Object.values(counts).reduce((sum, count) => sum + count, 0);
  const maxSeverity = SEVERITY_ORDER.findLast((severity) => (counts[severity] ?? 0) > 0) ?? "minor";
  return { type, counts, total, maxSeverity };
}

/**
 * One row per hazard type present, most severe first, then the most events, then by name. Types
 * this build doesn't know are summed into the `other` row.
 */
function matrix(summary: Pick<CountrySummary, "counts">): MatrixRow[] {
  const byType = new Map<EventType, Partial<Record<Severity, number>>>();
  for (const cell of summary.counts) {
    const type = eventType(cell.type);
    const counts = byType.get(type) ?? {};
    counts[cell.severity] = (counts[cell.severity] ?? 0) + cell.count;
    byType.set(type, counts);
  }
  return [...byType]
    .map(([type, counts]) => row(type, counts))
    .toSorted(
      (left, right) =>
        compareSeverity(right.maxSeverity, left.maxSeverity) ||
        right.total - left.total ||
        typeLabel(left.type).localeCompare(typeLabel(right.type), "en"),
    );
}

/** The listed events in one matrix cell, or all of them. */
function eventsIn<Event extends Pick<HazardEvent, "type" | "severity">>(
  events: readonly Event[],
  cell?: MatrixCell,
): Event[] {
  if (cell === undefined) {
    return [...events];
  }
  return events.filter((event) => eventType(event.type) === cell.type && event.severity === cell.severity);
}

/** The events a cell holds in all, from the counts (the list may hold fewer). */
function countIn(summary: Pick<CountrySummary, "counts">, cell: MatrixCell): number {
  return matrix(summary).find((candidate) => candidate.type === cell.type)?.counts[cell.severity] ?? 0;
}

export type { Country, CountrySummary, MatrixCell, MatrixRow };
export {
  countIn,
  EVENT_LIMIT,
  eventsIn,
  highSeverityCount,
  isListCapped,
  mapBox,
  matching,
  matrix,
  normalizedCode,
  smallestArc,
  warningCount,
};
// Revision words and the event frame, where the brief for this module put them.
export type { Change, ChangeStyle, EventRevision, RecentChange } from "./changes.ts";
export { changeSentence, isNotable, notableChanges, parseChanges, timeline } from "./changes.ts";
export { eventShape, frame } from "./geo.ts";

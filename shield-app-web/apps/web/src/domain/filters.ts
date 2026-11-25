import type { EventKind, HazardEvent, paths, Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";

import { kindTokens } from "@/design/tokens.gen.ts";

import type { EventType } from "./taxonomy.ts";
import { CONFLICT, eventType, HAZARD_TYPES, isAtLeast, kindPluralLabel, severityLabel, typeLabel } from "./taxonomy.ts";

/** What the UI asks for, and the chips that say how it differs from the default (DESIGN.md §10.6). */

const MS_PER_SECOND = 1000;
const SECONDS_PER_HOUR = 3600;
const HOURS_PER_DAY = 24;
/** From two days up, a whole number of days reads in days: "7 d". */
const DAYS_FROM_HOURS = 48;
/** Up to this many chosen types are named on the chip; more are counted. */
const NAMED_TYPES = 2;

const HOUR = SECONDS_PER_HOUR;
const SIX_HOURS = 21_600;
const DAY = 86_400;
const WEEK = 604_800;
/** The time windows the filter sheet offers: 1 h · 6 h · 24 h · 7 d, in seconds. */
const WINDOWS: readonly number[] = [HOUR, SIX_HOURS, DAY, WEEK];
const DEFAULT_WINDOW = DAY;

/**
 * The kinds a query can hold. Satellite heat detections are the map's FIRMS layer, not events, so
 * they are never part of `kinds`.
 */
const EVENT_KINDS: readonly EventKind[] = ["observed", "warning", "forecast"];

/** `GET /v1/events` filters. `windowSeconds` undefined means any time; `country` is a place, not a filter. */
interface EventQuery {
  readonly types: ReadonlySet<EventType>;
  readonly minimumSeverity: Severity;
  readonly kinds: ReadonlySet<EventKind>;
  readonly includeResolved: boolean;
  readonly windowSeconds: number | undefined;
  readonly country: string | undefined;
}

/** One dimension of `EventQuery` that differs from the default, as a removable chip. */
type ActiveFilter = "types" | "severity" | "kinds" | "window" | "status";
/** A stat-strip cell that filters when tapped, and unfilters when tapped again (DESIGN.md §10.1). */
type QuickFilter = "highSeverity" | "warnings";

const ACTIVE_FILTERS: readonly ActiveFilter[] = ["types", "severity", "kinds", "window", "status"];

/** Every hazard type, minor and up, observed + warnings + forecasts, last 24 h, active only. */
const DEFAULT_QUERY: EventQuery = {
  types: new Set(HAZARD_TYPES),
  minimumSeverity: "minor",
  kinds: new Set(EVENT_KINDS),
  includeResolved: false,
  windowSeconds: DEFAULT_WINDOW,
  country: undefined,
};

/**
 * The conflicts layer (ADR 27): every conflict the server still shows. No time window: the server
 * keeps each one 90 days after its end date, and the dates are months old.
 */
const CONFLICTS_QUERY: EventQuery = {
  ...DEFAULT_QUERY,
  types: new Set([CONFLICT]),
  kinds: new Set(["observed"]),
  windowSeconds: undefined,
};

/** True when both sets hold the same members. */
function sameSet<Item>(left: ReadonlySet<Item>, right: ReadonlySet<Item>): boolean {
  return left.size === right.size && [...left].every((item) => right.has(item));
}

/**
 * Whether `event` passes the query. A resolved event is windowed by when it ended (else its last
 * update), an active one by its last update: the server's rule (PR #23).
 */
function matches(query: EventQuery, event: HazardEvent, now: Date = new Date()): boolean {
  if (!query.types.has(eventType(event.type)) || !query.kinds.has(event.kind)) {
    return false;
  }
  if (!isAtLeast(event.severity, query.minimumSeverity)) {
    return false;
  }
  const resolved = event.status === "resolved";
  if (resolved && !query.includeResolved) {
    return false;
  }
  const windowed = Date.parse(resolved ? (event.endedAt ?? event.updatedAt) : event.updatedAt);
  if (query.windowSeconds !== undefined && (now.getTime() - windowed) / MS_PER_SECOND > query.windowSeconds) {
    return false;
  }
  return query.country === undefined || event.countries.some((country) => country.iso2 === query.country);
}

const DIFFERS: Readonly<Record<ActiveFilter, (query: EventQuery) => boolean>> = {
  types: (query) => !sameSet(query.types, DEFAULT_QUERY.types),
  severity: (query) => query.minimumSeverity !== DEFAULT_QUERY.minimumSeverity,
  kinds: (query) => !sameSet(query.kinds, DEFAULT_QUERY.kinds),
  window: (query) => query.windowSeconds !== DEFAULT_QUERY.windowSeconds,
  status: (query) => query.includeResolved !== DEFAULT_QUERY.includeResolved,
};

const RESETS: Readonly<Record<ActiveFilter, (query: EventQuery) => EventQuery>> = {
  types: (query) => ({ ...query, types: DEFAULT_QUERY.types }),
  severity: (query) => ({ ...query, minimumSeverity: DEFAULT_QUERY.minimumSeverity }),
  kinds: (query) => ({ ...query, kinds: DEFAULT_QUERY.kinds }),
  window: (query) => ({ ...query, windowSeconds: DEFAULT_QUERY.windowSeconds }),
  status: (query) => ({ ...query, includeResolved: DEFAULT_QUERY.includeResolved }),
};

/** The dimensions that differ from the default, in chip order. */
function activeFilters(query: EventQuery): ActiveFilter[] {
  return ACTIVE_FILTERS.filter((filter) => DIFFERS[filter](query));
}

/** No filter set. The country is a place, not a filter, and doesn't count. */
function isDefault(query: EventQuery): boolean {
  return activeFilters(query).length === 0;
}

/** This query with one dimension back at its default (the chip's ×). */
function removing(query: EventQuery, filter: ActiveFilter): EventQuery {
  return RESETS[filter](query);
}

function capitalizeFirst(text: string): string {
  return `${text.slice(0, 1).toUpperCase()}${text.slice(1)}`;
}

/** "1 h", "6 h", "24 h", "7 d". */
function windowLabel(seconds: number): string {
  const hours = Math.round(seconds / SECONDS_PER_HOUR);
  return hours >= DAYS_FROM_HOURS && hours % HOURS_PER_DAY === 0 ? `${hours / HOURS_PER_DAY} d` : `${hours} h`;
}

/** "Earthquake", "Flood, Storm", "5 types": hazards only, in the apps' type order. */
function typesLabel(types: ReadonlySet<EventType>): string {
  const chosen = HAZARD_TYPES.filter((type) => types.has(type));
  return chosen.length <= NAMED_TYPES ? chosen.map((type) => typeLabel(type)).join(", ") : `${chosen.length} types`;
}

/** "Warnings only", "Observed + forecasts". */
function kindsLabel(kinds: ReadonlySet<EventKind>): string {
  const chosen = EVENT_KINDS.filter((kind) => kinds.has(kind)).map((kind) => kindPluralLabel(kind));
  const [only] = chosen;
  return chosen.length === 1 && only !== undefined
    ? `${capitalizeFirst(only)} only`
    : capitalizeFirst(chosen.join(" + "));
}

const LABELS: Readonly<Record<ActiveFilter, (query: EventQuery) => string>> = {
  types: (query) => typesLabel(query.types),
  severity: (query) => `${severityLabel(query.minimumSeverity)}+`,
  kinds: (query) => kindsLabel(query.kinds),
  window: (query) => (query.windowSeconds === undefined ? "Any time" : windowLabel(query.windowSeconds)),
  status: (query) => (query.includeResolved ? "With resolved" : "Active only"),
};

/** The chip's words: "Earthquake", "5 types", "Severe+", "Warnings only", "7 d", "With resolved". */
function filterLabel(query: EventQuery, filter: ActiveFilter): string {
  return LABELS[filter](query);
}

/** Whether a stat-strip cell's filter is on: severe and up, or official warnings only. */
function isOn(query: EventQuery, quick: QuickFilter): boolean {
  return quick === "highSeverity"
    ? query.minimumSeverity === "severe"
    : sameSet(query.kinds, new Set<EventKind>(["warning"]));
}

/** On if it was off; back to that dimension's default if it was on. Each cell touches only its own. */
function toggling(query: EventQuery, quick: QuickFilter): EventQuery {
  const on = isOn(query, quick);
  if (quick === "highSeverity") {
    return { ...query, minimumSeverity: on ? DEFAULT_QUERY.minimumSeverity : "severe" };
  }
  return { ...query, kinds: on ? DEFAULT_QUERY.kinds : new Set<EventKind>(["warning"]) };
}

type ListEventsQuery = NonNullable<paths["/v1/events"]["get"]["parameters"]["query"]>;

/** Sorted values as CSV, or undefined when every value is chosen (the server's default). */
function csv(values: readonly string[], all: number): string | undefined {
  return values.length === all ? undefined : values.toSorted().join(",");
}

/**
 * The `/v1/events` query params for `query`. `type` is left out for every hazard type, so types the
 * server adds later still show; conflicts are sent only when named (ADR 27).
 */
function toApiQuery(query: EventQuery, now: Date = new Date()): ListEventsQuery {
  const types = sameSet(query.types, DEFAULT_QUERY.types) ? undefined : [...query.types].toSorted().join(",");
  const severity = csv(
    SEVERITY_ORDER.filter((level) => isAtLeast(level, query.minimumSeverity)),
    SEVERITY_ORDER.length,
  );
  const kind = csv([...query.kinds], Object.keys(kindTokens).length);
  const since =
    query.windowSeconds === undefined
      ? undefined
      : new Date(now.getTime() - query.windowSeconds * MS_PER_SECOND).toISOString();
  return {
    ...(types === undefined ? {} : { type: types }),
    ...(severity === undefined ? {} : { severity }),
    ...(kind === undefined ? {} : { kind }),
    status: query.includeResolved ? "active,resolved" : "active",
    ...(query.country === undefined ? {} : { country: query.country }),
    ...(since === undefined ? {} : { since }),
  };
}

export type { ActiveFilter, EventQuery, ListEventsQuery, QuickFilter };
export {
  ACTIVE_FILTERS,
  activeFilters,
  CONFLICTS_QUERY,
  DEFAULT_QUERY,
  EVENT_KINDS,
  filterLabel,
  isDefault,
  isOn,
  matches,
  removing,
  toApiQuery,
  toggling,
  WINDOWS,
  windowLabel,
};

/**
 * What changed in a revision of an event, in words: a country's recent changes and an event's
 * timeline (`store/revisions.go`). Mirrors shield-core `Change`, `RecentChange` (Countries.swift)
 * and `EventDetail.timeline` (EventDetail.swift).
 */
import type { components, Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";

import { compareSeverity, severityLabel } from "./taxonomy.ts";

type EventStatus = components["schemas"]["EventStatus"];
type Revision = components["schemas"]["Revision"];
type WireRecentChange = components["schemas"]["CountrySummary"]["recentChanges"][number];

/** One field of a revision. Values the server may add later are dropped, never guessed at. */
type Change =
  | { field: "severity"; from: Severity | undefined; to: Severity }
  | { field: "status"; from: EventStatus | undefined; to: EventStatus }
  /** The source's own words, verbatim. */
  | { field: "sourceSeverity"; from: string | undefined; to: string | undefined }
  | { field: "geometry" }
  | { field: "title"; from: string | undefined; to: string | undefined }
  | { field: "endedAt"; to: Date | undefined };

/** `summary` for a country's recent changes; `timeline` for one event's history, which names both levels. */
type ChangeStyle = "summary" | "timeline";

/** One revision of an event in a country's recent changes. */
interface RecentChange {
  eventId: string;
  /** The event's title now. */
  title: string;
  at: Date;
  /** Never empty for a change worth listing. */
  changes: Change[];
}

/** One revision in an event's timeline. */
interface EventRevision {
  at: Date;
  /** Never empty for a revision worth showing. */
  changes: Change[];
}

/** Reading order: what matters most first. */
const RANK: Readonly<Record<Change["field"], number>> = {
  severity: 0,
  status: 1,
  sourceSeverity: 2,
  geometry: 3,
  title: 4,
  endedAt: 5,
};

const STATUSES: readonly EventStatus[] = ["active", "resolved"];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function text(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function severity(value: unknown): Severity | undefined {
  return SEVERITY_ORDER.find((known) => known === value);
}

function status(value: unknown): EventStatus | undefined {
  return STATUSES.find((known) => known === value);
}

function date(value: unknown): Date | undefined {
  const raw = text(value);
  if (raw === undefined) {
    return undefined;
  }
  const parsed = new Date(raw);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed;
}

type Reader = (pair: Readonly<Record<string, unknown>>) => Change | undefined;

/** Each field's reader, in reading order; a reader answers undefined for a value it can't read. */
const READERS: readonly (readonly [string, Reader])[] = [
  [
    "severity",
    (pair) => {
      const to = severity(pair["to"]);
      return to && { field: "severity", from: severity(pair["from"]), to };
    },
  ],
  [
    "status",
    (pair) => {
      const to = status(pair["to"]);
      return to && { field: "status", from: status(pair["from"]), to };
    },
  ],
  ["sourceSeverity", (pair) => ({ field: "sourceSeverity", from: text(pair["from"]), to: text(pair["to"]) })],
  ["geometry", (pair) => (pair["changed"] === true ? { field: "geometry" } : undefined)],
  ["title", (pair) => ({ field: "title", from: text(pair["from"]), to: text(pair["to"]) })],
  ["endedAt", (pair) => ({ field: "endedAt", to: date(pair["to"]) })],
];

/**
 * The `changes` object of a revision: `{"severity": {"from": "moderate", "to": "severe"},
 * "geometry": {"changed": true}, ...}`. Unknown keys and unreadable values are skipped.
 */
function parseChanges(object: Readonly<Record<string, unknown>>): Change[] {
  return READERS.flatMap(([key, read]) => {
    const pair = object[key];
    return isRecord(pair) ? (read(pair) ?? []) : [];
  });
}

function severityWords(from: Severity | undefined, to: Severity, style: ChangeStyle): string {
  const level = severityLabel(to);
  if (from === undefined || from === to) {
    return style === "timeline" ? `severity now ${level}` : `now ${level}`;
  }
  if (style === "timeline") {
    return `severity ${severityLabel(from)} → ${level}`;
  }
  return compareSeverity(to, from) > 0 ? `raised to ${level}` : `lowered to ${level}`;
}

function statusWords(from: EventStatus | undefined, to: EventStatus): string | undefined {
  if (to === "resolved") {
    return "resolved";
  }
  return from === "resolved" ? "active again" : undefined;
}

/**
 * The words for one change, lower case; undefined when another change in the same revision says
 * it already (a status change covers its end time).
 */
function changeWords(change: Change, all: readonly Change[], style: ChangeStyle = "summary"): string | undefined {
  switch (change.field) {
    case "severity": {
      return severityWords(change.from, change.to, style);
    }
    case "status": {
      return statusWords(change.from, change.to);
    }
    case "sourceSeverity": {
      return change.to === undefined || change.to === "" ? "source severity cleared" : `source now says “${change.to}”`;
    }
    case "geometry": {
      return "area updated";
    }
    case "title": {
      return "renamed";
    }
    case "endedAt": {
      if (all.some((other) => other.field === "status")) {
        return undefined;
      }
      return change.to === undefined ? "end time removed" : "end time updated";
    }
    default: {
      return undefined;
    }
  }
}

/** The changes as one line, most important first, capitalised: "Raised to Severe · area updated". */
function changeSentence(changes: readonly Change[], style: ChangeStyle = "summary"): string {
  const parts = changes
    .toSorted((left, right) => RANK[left.field] - RANK[right.field])
    .flatMap((change) => changeWords(change, changes, style) ?? []);
  const [first, ...rest] = parts;
  if (first === undefined) {
    return "Updated";
  }
  return [`${first.charAt(0).toUpperCase()}${first.slice(1)}`, ...rest].join(" · ");
}

/**
 * Worth a line: a revision that only renamed its event is left out. MeteoAlarm titles gained their
 * area in S1.5, and fifty "renamed" rows buried the one severity change. Empty ones go too.
 */
function isNotable(changes: readonly Change[]): boolean {
  return changes.some((change) => change.field !== "title");
}

/** A country's recent changes worth a line, newest first as the server sends them. */
function notableChanges(recentChanges: readonly WireRecentChange[]): RecentChange[] {
  return recentChanges
    .map((change) => ({
      eventId: change.eventId,
      title: change.title,
      at: new Date(change.at),
      changes: parseChanges(change.changes),
    }))
    .filter((change) => isNotable(change.changes));
}

/** The revisions worth a line in an event's timeline (DESIGN.md §10.3 item 7). */
function timeline(revisions: readonly Revision[]): EventRevision[] {
  return revisions
    .map((revision) => ({ at: new Date(revision.at), changes: parseChanges(revision.changes) }))
    .filter((revision) => isNotable(revision.changes));
}

export type { Change, ChangeStyle, EventRevision, RecentChange };
export { changeSentence, changeWords, isNotable, notableChanges, parseChanges, timeline };

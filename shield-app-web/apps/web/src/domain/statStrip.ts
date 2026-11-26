/**
 * The live map's stat strip (DESIGN.md §10.1): observed events, high severity and official
 * warnings for one response of `/v1/events`. Mirrors shield-apple `StatStrip.Cell.liveMap`.
 */
import type { EventCluster, EventList } from "@shield/api-client";

import { isAtLeast } from "./taxonomy.ts";

/** The quick filter a cell toggles when tapped. */
type StatFilter = "highSeverity" | "warnings";

/** One cell: the number as shown ("12", "12+", "–") and its words. */
interface StatCell {
  value: string;
  label: string;
  filter?: StatFilter;
}

/** A count and whether the real number is at least that ("12+"); undefined when unknown ("–"). */
interface Count {
  value: number | undefined;
  isLowerBound: boolean;
}

const UNKNOWN = "–";
const LABELS = { observed: "Observed events", high: "High severity", warnings: "Official warnings" } as const;

function text({ value, isLowerBound }: Count): string {
  if (value === undefined) {
    return UNKNOWN;
  }
  return isLowerBound ? `${value}+` : String(value);
}

/**
 * High-severity events inside clusters. A cluster's severity split is exact; without one, all that
 * is known is whether its worst member is high, so it counts once, as a lower bound.
 */
function clusteredHigh(clusters: readonly EventCluster[]): { value: number; isLowerBound: boolean } {
  let value = 0;
  let isLowerBound = false;
  for (const cluster of clusters) {
    if (Object.keys(cluster.severities).length > 0) {
      value += (cluster.severities["severe"] ?? 0) + (cluster.severities["extreme"] ?? 0);
    } else if (isAtLeast(cluster.maxSeverity, "severe")) {
      value += 1;
      isLowerBound ||= cluster.count > 1;
    }
  }
  return { value, isLowerBound };
}

/**
 * The three cells for one response of the events endpoint; dashes before one arrives.
 *
 * Counts include the events inside clusters, so the strip agrees with the map at every zoom.
 * A cluster carries a total and a count per severity, but nothing about kind, so warnings are a
 * lower bound ("12+") wherever clusters are on screen, or a dash when none is known. Never a guess.
 *
 * The strip counts what is happening, so resolved events never count. Clusters don't say how many
 * of theirs are resolved, so while resolved events are included, the counts that need clusters
 * show a dash.
 */
function liveMapCells(
  list: Pick<EventList, "events" | "clusters" | "truncated"> | undefined,
  includesResolved = false,
): StatCell[] {
  if (list === undefined) {
    return [LABELS.observed, LABELS.high, LABELS.warnings].map((label) => ({ value: UNKNOWN, label }));
  }
  const active = list.events.filter((event) => event.status !== "resolved");
  const clustered = list.clusters.length > 0;
  const mixesStatuses = includesResolved && clustered;
  // More matched than the server sent, so every count is short.
  const short = list.truncated;
  const inClusters = clusteredHigh(mixesStatuses ? [] : list.clusters);
  const observed = active.length + list.clusters.reduce((sum, cluster) => sum + cluster.count, 0);
  const high = active.filter((event) => isAtLeast(event.severity, "severe")).length + inClusters.value;
  const warnings = active.filter((event) => event.kind === "warning").length;
  return [
    { value: text({ value: mixesStatuses ? undefined : observed, isLowerBound: short }), label: LABELS.observed },
    {
      value: text({ value: mixesStatuses ? undefined : high, isLowerBound: inClusters.isLowerBound || short }),
      label: LABELS.high,
      filter: "highSeverity",
    },
    {
      value: text({ value: clustered && warnings === 0 ? undefined : warnings, isLowerBound: clustered || short }),
      label: LABELS.warnings,
      filter: "warnings",
    },
  ];
}

type Counted = Pick<EventList, "events" | "clusters" | "truncated">;

/**
 * The landing page's strip: the whole world under the default filters, which cells don't filter.
 * Clusters say nothing about kind, so official warnings come from their own `kind=warning` answer,
 * where every event and every clustered one is a warning: an exact count, not the map's "12+".
 */
function worldCells(all: Counted | undefined, warnings: Counted | undefined): StatCell[] {
  const cells = liveMapCells(all).map(({ value, label }) => ({ value, label }));
  const counted =
    warnings === undefined
      ? undefined
      : warnings.events.filter((event) => event.status !== "resolved").length +
        warnings.clusters.reduce((sum, cluster) => sum + cluster.count, 0);
  const last = { value: text({ value: counted, isLowerBound: warnings?.truncated ?? false }), label: LABELS.warnings };
  return [...cells.slice(0, -1), last];
}

/** The compact strip's words after each number: singular and plural (StatStrip.swift `units`). */
const UNITS: Readonly<Record<string, { one: string; many: string }>> = {
  [LABELS.observed]: { one: "event", many: "events" },
  [LABELS.high]: { one: "high severity", many: "high severity" },
  [LABELS.warnings]: { one: "warning", many: "warnings" },
};

/** "4 events", "1 warning", "– warnings": the compact strip at the sheet's peek. */
function compactWords(cell: StatCell): string {
  const unit = UNITS[cell.label] ?? { one: cell.label.toLowerCase(), many: cell.label.toLowerCase() };
  return cell.value === "1" ? unit.one : unit.many;
}

/** "High severity: 3", "Official warnings: at least 12", "Official warnings: not counted at this zoom". */
function cellDescription(cell: StatCell): string {
  if (cell.value === UNKNOWN) {
    return `${cell.label}: not counted at this zoom`;
  }
  return cell.value.endsWith("+")
    ? `${cell.label}: at least ${cell.value.slice(0, -1)}`
    : `${cell.label}: ${cell.value}`;
}

export type { StatCell, StatFilter };
export { cellDescription, compactWords, liveMapCells, UNKNOWN, worldCells };

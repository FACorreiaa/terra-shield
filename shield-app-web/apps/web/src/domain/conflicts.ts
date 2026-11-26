import type { EventList } from "@shield/api-client";

/**
 * The armed-conflicts layer's words (ADR 27; `ConflictLegend` in MapScreen.swift, `conflictSection`
 * in EventList.swift). UCDP data are monthly research estimates, so the layer never says "live",
 * "updated" or "stale".
 */

/** Always under the legend's headline, whatever its state. */
const CONFLICT_NOTE = "Conflict data are monthly research estimates (UCDP), not live reports.";

/** The list section's header. */
const CONFLICT_SECTION = "Armed conflicts · past 90 days";

/** Conflicts in an answer: the listed events plus every cluster's count (MapSnapshot `totalCount`). */
function conflictCount(list: Pick<EventList, "events" | "clusters">): number {
  let count = list.events.length;
  for (const cluster of list.clusters) {
    count += cluster.count;
  }
  return count;
}

interface ConflictLegendState {
  /** The answer shown; undefined before the first one since the layer was turned on. */
  list: Pick<EventList, "events" | "clusters" | "truncated"> | undefined;
  failed: boolean;
}

/** "Armed conflicts, past 90 days · 12 in this view", or how it is loading or failed. */
function conflictHeadline({ list, failed }: ConflictLegendState): string {
  if (failed) {
    return "Armed conflicts didn’t load";
  }
  if (list === undefined) {
    return "Armed conflicts · loading…";
  }
  const count = conflictCount(list);
  const shown = count === 0 ? "none in this view" : `${count} in this view`;
  return `Armed conflicts, past 90 days · ${shown}${list.truncated ? " · zoom in for all" : ""}`;
}

/** The list section's line under its header: what the data are, and what the map grouped. */
function conflictSectionNote(clustered: number): string {
  return clustered > 0
    ? `Monthly research estimates (UCDP). ${clustered} more grouped on the map; zoom in to list them.`
    : "Monthly research estimates (UCDP), not live reports.";
}

export { CONFLICT_NOTE, CONFLICT_SECTION, conflictCount, conflictHeadline, conflictSectionNote };

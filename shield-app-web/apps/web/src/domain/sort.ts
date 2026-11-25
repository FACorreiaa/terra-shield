import type { HazardEvent } from "@shield/api-client";

import { compareSeverity } from "./taxonomy.ts";

type Sortable = Pick<HazardEvent, "status" | "severity" | "updatedAt">;

/**
 * Display order everywhere (DESIGN.md §4): active before resolved, most severe first, then most
 * recently updated. A resolved severe event matters less now than an active moderate one.
 */
function compareForDisplay(left: Sortable, right: Sortable): number {
  const leftResolved = left.status === "resolved";
  const rightResolved = right.status === "resolved";
  if (leftResolved !== rightResolved) {
    return leftResolved ? 1 : -1;
  }
  const bySeverity = compareSeverity(right.severity, left.severity);
  if (bySeverity !== 0) {
    return bySeverity;
  }
  return Date.parse(right.updatedAt) - Date.parse(left.updatedAt);
}

/** A sorted copy for display; the input is left as it was. */
function sortedForDisplay<Item extends Sortable>(events: readonly Item[]): Item[] {
  return events.toSorted(compareForDisplay);
}

export { compareForDisplay, sortedForDisplay };

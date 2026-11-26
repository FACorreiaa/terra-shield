import type { EventKind } from "@shield/api-client";
import { useState } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import { DEFAULT_QUERY } from "@/domain/filters.ts";
import type { StatCell } from "@/domain/statStrip.ts";
import { worldCells } from "@/domain/statStrip.ts";
import { failureWords, useEvents } from "@/features/events/useEvents.ts";

/** The whole world, clustered as the map is at its widest. */
const WORLD = { zoom: 1 } as const;
const WARNINGS: EventQuery = { ...DEFAULT_QUERY, kinds: new Set<EventKind>(["warning"]) };

interface WorldCounts {
  cells: StatCell[];
  /** When the numbers arrived, by this browser's clock: "Updated 2 min ago". */
  updatedAt: Date | undefined;
  /** Why a number is missing, in the map's own words ("Shield’s server isn’t responding."). */
  failure: string | undefined;
}

/**
 * Why a number is missing, kept while it is asked for again: a query with no data goes back to
 * pending on each retry, which would flip the line to "Loading…" every minute.
 */
function useLastFailure(missing: boolean, error: Error | null): string | undefined {
  const [last, setLast] = useState<string>();
  const now = error === null ? undefined : failureWords(error);
  const next = missing ? (now ?? last) : undefined;
  if (next !== last) {
    setLast(next);
  }
  return next;
}

/**
 * The landing page's numbers: live, from `/v1/events` under the default filters, never sample
 * numbers. Two answers, as warnings in clusters can't be told from the first one (statStrip.ts).
 * Either one failing says why under the strip.
 */
function useWorldCounts(): WorldCounts {
  const all = useEvents(WORLD, DEFAULT_QUERY);
  const warnings = useEvents(WORLD, WARNINGS);
  const missing = all.data === undefined || warnings.data === undefined;
  const failure = useLastFailure(missing, all.error ?? warnings.error);
  return {
    cells: worldCells(all.data, warnings.data),
    updatedAt: all.dataUpdatedAt === 0 ? undefined : new Date(all.dataUpdatedAt),
    failure,
  };
}

export { useWorldCounts };

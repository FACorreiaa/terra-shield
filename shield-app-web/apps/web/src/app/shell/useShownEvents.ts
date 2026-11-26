import type { EventCluster, EventList, HazardEvent } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { onlineManager } from "@tanstack/react-query";
import { useMemo, useState, useSyncExternalStore } from "react";

import type { LiveStatus } from "@/domain/liveStatus.ts";
import { liveStatus } from "@/domain/liveStatus.ts";
import { sortedForDisplay } from "@/domain/sort.ts";
import { LoadFailureError } from "@/features/events/useEvents.ts";

interface Shown {
  /** The current response, else the last one that arrived. */
  shown: EventList | undefined;
  /** The capsule's words and colours. */
  status: LiveStatus;
  /** Why the last request failed, in words for people, while online. */
  failure: string | undefined;
}

const SOMETHING_WRONG = "Something went wrong loading events.";

/** The failure in words for people; anything not written for them reads as a plain failure. */
function failureWords(error: unknown): string {
  return error instanceof LoadFailureError ? error.message : SOMETHING_WRONG;
}

/** The browser's online state as TanStack Query sees it: offline, requests wait instead of failing. */
function useOnline(): boolean {
  return useSyncExternalStore(
    (listener) => onlineManager.subscribe(listener),
    () => onlineManager.isOnline(),
    () => true,
  );
}

/** The current response, else the last one that arrived, and when that was (ms since the epoch). */
function useLastGood(events: UseQueryResult<EventList>): { list: EventList | undefined; at: number | undefined } {
  const [last, setLast] = useState<{ list: EventList; at: number }>();
  const fresh = events.isSuccess && !events.isPlaceholderData;
  // By identity, not time: a live patch keeps the load's time but is a newer list.
  if (fresh && (events.data !== last?.list || events.dataUpdatedAt !== last.at)) {
    setLast({ list: events.data, at: events.dataUpdatedAt });
  }
  return { list: events.data ?? last?.list, at: last?.at };
}

/**
 * Keeps what the map shows through a failed or pending request (DESIGN.md §10.1: nothing is
 * cleared): a request for a new view or new filters that fails has no data of its own, so the last
 * response that arrived stays on screen. Says how loading is going, for the capsule and the banner.
 */
function useShownEvents(events: UseQueryResult<EventList>, now: Date, liveAt: Date | undefined): Shown {
  const online = useOnline();
  const { list, at } = useLastGood(events);
  const failed = online && events.isError;
  const loading = events.isFetching;
  const status = useMemo(
    () =>
      liveStatus({
        lastSuccess: at === undefined ? undefined : new Date(at),
        loading,
        offline: !online,
        failed,
        liveAt,
        now,
      }),
    [at, loading, online, failed, liveAt, now],
  );
  return { shown: list, status, failure: failed ? failureWords(events.error) : undefined };
}

const NO_EVENTS: readonly HazardEvent[] = [];
const NO_CLUSTERS: readonly EventCluster[] = [];

/** One order for the map's list and for ↑/↓: most severe, then newest, resolved last. */
function useDisplayed(shown: EventList | undefined): {
  list: readonly HazardEvent[];
  clusters: readonly EventCluster[];
} {
  const list = useMemo(() => (shown === undefined ? NO_EVENTS : sortedForDisplay(shown.events)), [shown]);
  return { list, clusters: shown?.clusters ?? NO_CLUSTERS };
}

export { useDisplayed, useShownEvents };

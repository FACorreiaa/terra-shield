import type { EventList } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { onlineManager } from "@tanstack/react-query";
import { useMemo, useState, useSyncExternalStore } from "react";

import type { LiveStatus } from "@/domain/liveStatus.ts";
import { liveStatus } from "@/domain/liveStatus.ts";
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
  if (fresh && events.dataUpdatedAt !== last?.at) {
    setLast({ list: events.data, at: events.dataUpdatedAt });
  }
  return { list: events.data ?? last?.list, at: last?.at };
}

/**
 * Keeps what the map shows through a failed or pending request (DESIGN.md §10.1: nothing is
 * cleared): a request for a new view or new filters that fails has no data of its own, so the last
 * response that arrived stays on screen. Says how loading is going, for the capsule and the banner.
 */
function useShownEvents(events: UseQueryResult<EventList>, now: Date): Shown {
  const online = useOnline();
  const { list, at } = useLastGood(events);
  const failed = online && events.isError;
  const loading = events.isFetching;
  const status = useMemo(
    () =>
      liveStatus({ lastSuccess: at === undefined ? undefined : new Date(at), loading, offline: !online, failed, now }),
    [at, loading, online, failed, now],
  );
  return { shown: list, status, failure: failed ? failureWords(events.error) : undefined };
}

export { useShownEvents };

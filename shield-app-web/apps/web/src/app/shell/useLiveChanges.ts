import type { EventList } from "@shield/api-client";
import type { QueryClient, UseQueryResult } from "@tanstack/react-query";
import { hashKey, useQueryClient } from "@tanstack/react-query";
import { useEffect, useEffectEvent, useMemo, useSyncExternalStore } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import type { Outcome } from "@/domain/livePatch.ts";
import { burstMeter, patchEvents } from "@/domain/livePatch.ts";
import type { View } from "@/domain/viewport.ts";
import type { DetailAnswer } from "@/features/detail/detailState.ts";
import { eventsKey } from "@/features/events/useEvents.ts";
import type { LiveChange, LiveUpdate } from "@/lib/live/liveChange.ts";
import type { FeedStatus } from "@/lib/live/liveFeed.ts";
import { liveFeed } from "@/lib/live/liveFeed.ts";

/** Two missed heartbeats, plus slack: a half-open socket (sleep, a network change) goes quiet, and silence ends live. */
const LIVE_SILENCE_MS = 45_000;
/** A change that needs the server (clusters, a cut list) reloads once for a run of them. */
const RELOAD_AFTER_MS = 500;
const NOT_STARTED: FeedStatus = { ready: false, lastActivity: undefined };

interface LiveChangesInput {
  view: View;
  query: EventQuery;
  events: UseQueryResult<EventList>;
  selectedId: string | undefined;
  now: Date;
}

function subscribe(listener: () => void): () => void {
  return liveFeed().onStatus(listener);
}

function snapshot(): FeedStatus {
  return liveFeed().status;
}

function serverSnapshot(): FeedStatus {
  return NOT_STARTED;
}

interface Reloader {
  /** Reloads `after` ms from the first request of a run (a reload already due stands); 0 is now. */
  schedule: (after: number, refetch: () => unknown) => void;
  cancel: () => void;
}

function reloader(): Reloader {
  let due: ReturnType<typeof setTimeout> | undefined = undefined;
  let latest: (() => unknown) | undefined = undefined;
  const run = (): void => {
    due = undefined;
    void latest?.();
  };
  const cancel = (): void => {
    clearTimeout(due);
    due = undefined;
  };
  return {
    schedule: (after, refetch) => {
      latest = refetch;
      if (after === 0) {
        cancel();
        run();
      } else {
        due ??= setTimeout(run, after);
      }
    },
    cancel,
  };
}

/** The events' reload, debounced, kept for the component's life. */
function useReload(): Reloader {
  const reload = useMemo(() => reloader(), []);
  useEffect(() => reload.cancel, [reload]);
  return reload;
}

interface Applying {
  client: QueryClient;
  input: LiveChangesInput;
  reload: Reloader;
  inBurst: () => boolean;
}

/**
 * Keeps an open detail current: a change to an event outside the map's events (a country's, one in a
 * cluster), or its removal, which the detail then shows as gone.
 */
function keepSelection(
  change: LiveChange,
  { client, input }: Pick<Applying, "client" | "input">,
  shown: EventList | undefined,
): void {
  const { selectedId } = input;
  if (selectedId === undefined) {
    return;
  }
  // A shown event's detail refetches itself when its patched copy's `updatedAt` moves.
  const outside = shown?.events.some((each) => each.id === selectedId) !== true;
  const changed = (change.kind === "upsert" || change.kind === "created") && change.event.id === selectedId;
  if (change.kind === "removed" && change.id === selectedId) {
    // Gone now, not after a refetch the HTTP cache might answer with the old 200 (LiveMapModel
    // sets `.gone`). Out of the list, the detail is asked for without a version: that key too.
    const gone: DetailAnswer = { found: false };
    client.setQueriesData({ queryKey: ["event", selectedId] }, gone);
    client.setQueryData(["event", selectedId, undefined], gone);
  } else if (changed && outside) {
    void client.invalidateQueries({ queryKey: ["event", selectedId] });
  }
}

/**
 * Only the current view's entry is patched: the others (a view or filters left a moment ago) are
 * marked stale, so going back reloads them rather than showing them as they were.
 */
function markOthersStale(client: QueryClient, current: readonly unknown[]): void {
  const hash = hashKey(current);
  void client.invalidateQueries({
    queryKey: ["events"],
    refetchType: "none",
    predicate: (query) => query.queryHash !== hash,
  });
}

interface Settling {
  client: QueryClient;
  key: readonly unknown[];
  updatedAt: number;
  reload: () => void;
}

/** A patch decided: written in place (the load's own time kept), or a reload asked for. */
function settle(outcome: Outcome, { client, key, updatedAt, reload }: Settling): void {
  if (outcome.kind === "patched") {
    // A patch isn't a load, and while live the stream's word dates the data.
    client.setQueryData(key, outcome.list, { updatedAt });
  } else if (outcome.kind === "reload") {
    reload();
  }
  if (outcome.kind !== "none") {
    markOthersStale(client, key);
  }
}

/** One update from the feed, applied to the cache entry for the current view and filters. */
function applyUpdate({ change }: LiveUpdate, { client, input, reload, inBurst }: Applying): void {
  if (change === undefined) {
    return;
  }
  const key = eventsKey(input.view, input.query);
  if (change.kind === "ready" || change.kind === "reset") {
    reload.schedule(0, input.events.refetch);
    markOthersStale(client, key);
    return;
  }
  const state = client.getQueryState<EventList>(key);
  keepSelection(change, { client, input }, state?.data);
  // Nothing loaded for this view yet: the load on its way has the change, or the next one will.
  if (state?.data === undefined) {
    return;
  }
  const normal = change.kind === "removed" ? change : { kind: "upsert" as const, event: change.event };
  const outcome = patchEvents(state.data, normal, { view: input.view, query: input.query, now: new Date(), inBurst });
  settle(outcome, {
    client,
    key,
    updatedAt: state.dataUpdatedAt,
    reload: () => {
      reload.schedule(RELOAD_AFTER_MS, input.events.refetch);
    },
  });
}

/**
 * Applies the live feed's changes to the map's events (LiveMapModel `apply`): patched in the query
 * cache where the response is complete, else reloaded; `ready` and `reset` reload, the stream being
 * open before the load so no change falls between. Returns when the stream last spoke, while live.
 */
function useLiveChanges(input: LiveChangesInput): Date | undefined {
  const client = useQueryClient();
  const status = useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  const reload = useReload();
  const inBurst = useMemo(() => burstMeter(), []);
  const apply = useEffectEvent((update: LiveUpdate) => {
    applyUpdate(update, { client, input, reload, inBurst });
  });
  useEffect(
    () =>
      liveFeed().onUpdate((update) => {
        apply(update);
      }),
    [],
  );
  const { ready, lastActivity } = status;
  const live = ready && lastActivity !== undefined && input.now.getTime() - lastActivity < LIVE_SILENCE_MS;
  return useMemo(() => (live ? new Date(lastActivity) : undefined), [live, lastActivity]);
}

export { applyUpdate, useLiveChanges };

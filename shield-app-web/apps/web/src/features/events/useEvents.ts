import type { EventList } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { keepPreviousData, useQuery } from "@tanstack/react-query";

import type { EventQuery, ListEventsQuery } from "@/domain/filters.ts";
import { toApiQuery } from "@/domain/filters.ts";
import { api } from "@/lib/api/client.ts";

interface EventsView {
  zoom: number;
  /** minLon,minLat,maxLon,maxLat */
  bbox?: string | undefined;
}

/** Below the server's `max-age=15` a refetch would only re-hit its cache (ADR 16). */
const STALE_MS = 15_000;
const REFETCH_MS = 60_000;

/**
 * A load failure in words for people (LiveMapModel `LoadFailure`): library and server errors
 * describe themselves with debug text, which never reaches the banner.
 */
class LoadFailureError extends Error {
  public override readonly name = "LoadFailureError";
}

const UNREACHABLE = "Shield’s server isn’t responding.";
const FAILED = "Something went wrong loading events.";

async function request(query: ListEventsQuery, signal: AbortSignal): Promise<EventList> {
  try {
    // `no-cache`: the server's `max-age` would answer a reload the stream asked for with the list
    // it's meant to replace (a URL without `since` repeats); the query cache already spares requests.
    const { data, error } = await api.GET("/v1/events", { params: { query }, signal, cache: "no-cache" });
    // The endpoint also serves GeoJSON (Accept: application/geo+json); JSON is what we ask for.
    if (error !== undefined || !("events" in data)) {
      throw new LoadFailureError(FAILED);
    }
    return data;
  } catch (error) {
    // `fetch` rejects with a TypeError when the host can't be reached; aborts pass through.
    if (error instanceof TypeError) {
      throw new LoadFailureError(UNREACHABLE, { cause: error });
    }
    throw error;
  }
}

/**
 * The filters as a cache key. `since` moves with the clock, so the key holds the window instead and
 * the request computes `since` when it is sent.
 */
function filterKey(query: EventQuery): readonly unknown[] {
  const { since: _moving, ...rest } = toApiQuery(query);
  return [rest, query.windowSeconds];
}

/** The cache key of the events for a view and filters; the live stream patches the entry in place. */
function eventsKey(view: EventsView, query: EventQuery): readonly unknown[] {
  return ["events", view.zoom, view.bbox, ...filterKey(query)];
}

/** `GET /v1/events` for the current view and filters. The server clusters below zoom 6 (ADR 9). */
function useEvents(view: EventsView, query: EventQuery): UseQueryResult<EventList> {
  return useQuery({
    queryKey: eventsKey(view, query),
    queryFn: ({ signal }) =>
      request(
        { ...toApiQuery(query), zoom: view.zoom, ...(view.bbox === undefined ? {} : { bbox: view.bbox }) },
        signal,
      ),
    placeholderData: keepPreviousData,
    staleTime: STALE_MS,
    refetchInterval: REFETCH_MS,
  });
}

export type { EventsView };
export { eventsKey, LoadFailureError, request as requestEvents, useEvents };

import type { EventList } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api/client.ts";

interface EventsQuery {
  zoom: number;
  /** minLon,minLat,maxLon,maxLat */
  bbox?: string | undefined;
}

/** Below the server's `max-age=15` a refetch would only re-hit its cache (ADR 16). */
const STALE_MS = 15_000;
const REFETCH_MS = 60_000;

async function fetchEvents({ zoom, bbox }: EventsQuery, signal: AbortSignal): Promise<EventList> {
  const { data, error } = await api.GET("/v1/events", {
    params: { query: { zoom, ...(bbox === undefined ? {} : { bbox }) } },
    signal,
  });
  if (error !== undefined) {
    throw new Error(error.reason);
  }
  // The endpoint also serves GeoJSON (Accept: application/geo+json); JSON is what we ask for.
  if (!("events" in data)) {
    throw new Error("Unexpected GeoJSON response");
  }
  return data;
}

/** `GET /v1/events` for the current view. The server clusters below zoom 6 (ADR 9). */
function useEvents(query: EventsQuery): UseQueryResult<EventList> {
  return useQuery({
    queryKey: ["events", query.zoom, query.bbox],
    queryFn: ({ signal }) => fetchEvents(query, signal),
    placeholderData: keepPreviousData,
    staleTime: STALE_MS,
    refetchInterval: REFETCH_MS,
  });
}

export type { EventsQuery };
export { useEvents };

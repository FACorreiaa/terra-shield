import type { UseQueryResult } from "@tanstack/react-query";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api/client.ts";

import type { DetailAnswer } from "./detailState.ts";

/** Below the server's `max-age=15` a refetch would only re-hit its cache (ADR 16). */
const STALE_MS = 15_000;
const NOT_FOUND = 404;
/** A malformed id in a shared link: as gone as an id the source dropped. */
const BAD_REQUEST = 400;

async function request(id: string, signal: AbortSignal): Promise<DetailAnswer> {
  // `no-cache`: a refetch after a live change must reach the server, not the HTTP cache's copy.
  const { data, response } = await api.GET("/v1/events/{id}", { params: { path: { id } }, signal, cache: "no-cache" });
  if (response.status === NOT_FOUND || response.status === BAD_REQUEST) {
    return { found: false };
  }
  if (data === undefined) {
    throw new Error(`The event's detail answered ${response.status}`);
  }
  return { found: true, detail: data };
}

/**
 * `GET /v1/events/{id}` for the selection: geometry, source record and revisions. `version` is the
 * map's copy's `updatedAt`, so a newer copy refetches; the detail on screen stays until the new
 * answer arrives, and a newer selection drops the older answer (LiveMapModel `loadSelectedArea`).
 */
function useEventDetail(id: string | undefined, version: string | undefined): UseQueryResult<DetailAnswer> {
  return useQuery({
    queryKey: ["event", id, version],
    queryFn: ({ signal }) => request(id ?? "", signal),
    enabled: id !== undefined,
    placeholderData: (previous, previousQuery) => (previousQuery?.queryKey[1] === id ? previous : undefined),
    staleTime: STALE_MS,
    retry: 1,
  });
}

export { useEventDetail };

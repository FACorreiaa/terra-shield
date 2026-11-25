import type { Source } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api/client.ts";

/** Health changes on the server's own poll cadence; once a minute keeps staleness true (DESIGN.md §10.7). */
const REFETCH_MS = 60_000;
const STALE_MS = 30_000;

async function request(signal: AbortSignal): Promise<Source[]> {
  const { data, response } = await api.GET("/v1/sources", { signal });
  if (data === undefined) {
    throw new Error(`The source list answered ${response.status}`);
  }
  return data.sources;
}

/**
 * `GET /v1/sources`: names, links and health, for source badges, stale fading and the Sources
 * screen.
 */
function useSources(): UseQueryResult<Source[]> {
  return useQuery({
    queryKey: ["sources"],
    queryFn: ({ signal }) => request(signal),
    staleTime: STALE_MS,
    refetchInterval: REFETCH_MS,
  });
}

export { useSources };

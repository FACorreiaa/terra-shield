import type { UseQueryResult } from "@tanstack/react-query";
import { useQuery } from "@tanstack/react-query";

import type { Country } from "@/domain/countries.ts";
import { api } from "@/lib/api/client.ts";

/** An hour: boundaries and counts here change with the server's data, not by the minute. */
const COUNTRIES_FRESH_MS = 3_600_000;

async function request(signal: AbortSignal): Promise<Country[]> {
  const { data, error } = await api.GET("/v1/countries", { signal });
  if (error !== undefined) {
    throw new Error("The country list didn’t load.");
  }
  return data.countries;
}

/**
 * `GET /v1/countries`: every country with a boundary, its active count and worst severity. Loaded
 * on first use (`enabled`), as the apps do when search opens; search works without it.
 */
function useCountries(enabled: boolean, { fresh = false }: { fresh?: boolean } = {}): UseQueryResult<Country[]> {
  return useQuery({
    queryKey: ["countries"],
    queryFn: ({ signal }) => request(signal),
    enabled,
    // The Countries section asks again on each visit: its counts should be current (MapScreen.swift).
    staleTime: fresh ? 0 : COUNTRIES_FRESH_MS,
  });
}

export { useCountries };

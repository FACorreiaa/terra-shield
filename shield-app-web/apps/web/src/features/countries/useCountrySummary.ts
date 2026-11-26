import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { normalizedCode } from "@/domain/countries.ts";
import { api } from "@/lib/api/client.ts";

import type { CountryState, SummaryAnswer } from "./countryState.ts";
import { countryState } from "./countryState.ts";

/** Below the server's cache, a refetch would only re-hit it (ADR 16). */
const STALE_MS = 15_000;
/** The overview follows the map's minute (CountryView `refresh(ifOlderThan: 60)`). */
const REFRESH_MS = 60_000;
const NOT_FOUND = 404;
const BAD_REQUEST = 400;

async function request(iso2: string, signal: AbortSignal): Promise<SummaryAnswer> {
  const { data, response } = await api.GET("/v1/countries/{iso2}/summary", { params: { path: { iso2 } }, signal });
  if (response.status === NOT_FOUND || response.status === BAD_REQUEST) {
    return { found: false };
  }
  if (data === undefined) {
    throw new Error(`The country's summary answered ${response.status}`);
  }
  return { found: true, summary: data };
}

interface CountrySummaryResult {
  /** "PT" for "pt"; undefined when the address isn't two letters. */
  code: string | undefined;
  state: CountryState;
  retry: () => void;
}

/**
 * `GET /v1/countries/{iso2}/summary`, keyed on the code alone so a failed refresh keeps the
 * summary on screen (the query keeps its data); a malformed code makes no request.
 */
function useCountrySummary(iso2: string): CountrySummaryResult {
  const code = normalizedCode(iso2);
  const query = useQuery({
    queryKey: ["country", code],
    queryFn: ({ signal }) => request(code ?? "", signal),
    enabled: code !== undefined,
    staleTime: STALE_MS,
    // A place with no boundary won't grow one in a minute.
    refetchInterval: (current) => (current.state.data?.found === false ? false : REFRESH_MS),
    retry: 1,
  });
  const { data, isError, isFetching, refetch } = query;
  // Retrying with nothing shown says "Loading…" again rather than the failure.
  const failed = isError && !(isFetching && data === undefined);
  return useMemo(
    () => ({
      code,
      state: countryState(code, data, failed),
      retry: () => {
        void refetch();
      },
    }),
    [code, data, failed, refetch],
  );
}

export { useCountrySummary };

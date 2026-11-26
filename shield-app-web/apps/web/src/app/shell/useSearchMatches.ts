import type { HazardEvent } from "@shield/api-client";
import { useMemo } from "react";

import type { Country } from "@/domain/countries.ts";
import { matching } from "@/domain/countries.ts";
import { eventsMatching } from "@/domain/search.ts";
import { useCountries } from "@/features/countries/useCountries.ts";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";

interface SearchMatches {
  countries: readonly Country[];
  events: readonly HazardEvent[];
  /** The events in view have loaded, so "no match" is true and not just "not yet". */
  eventsLoaded: boolean;
  /** Events grouped on the map, which search can't see. */
  clustered: number;
}

const NO_COUNTRIES: readonly Country[] = [];

/** What the search's text finds: countries (loaded once searching starts) and events in view, in the list's order. */
function useSearchMatches(): SearchMatches {
  const { text, active } = useMapSearch();
  const { list, shown, clusters } = useLiveMap();
  const all = useCountries(active).data ?? NO_COUNTRIES;
  const countries = useMemo(() => matching(text, all), [text, all]);
  const events = useMemo(() => eventsMatching(text, list), [text, list]);
  const clustered = useMemo(() => clusters.reduce((sum, cluster) => sum + cluster.count, 0), [clusters]);
  return { countries, events, eventsLoaded: shown !== undefined, clustered };
}

export type { SearchMatches };
export { useSearchMatches };

import { useNavigate } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { useCallback } from "react";

import type { Country } from "@/domain/countries.ts";
import { SearchResults } from "@/features/search/SearchResults.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import { useIsWide } from "./useBreakpoint.ts";
import { focusRow } from "./useDetailFocus.ts";
import { reopenCountry, usePage } from "./usePage.ts";
import { useSearchMatches } from "./useSearchMatches.ts";

/**
 * Search's results, in place of the list while searching. Choosing an event selects it; choosing a
 * country opens its overview.
 */
function SearchPanel(): ReactElement {
  const { text, end } = useMapSearch();
  const { selectedId, select, now } = useLiveMap();
  const { countries, events, eventsLoaded, clustered } = useSearchMatches();
  const wide = useIsWide();
  const chooseEvent = useCallback(
    (id: string) => {
      end();
      select(id);
      // On a wide screen the keyboard carries on from the chosen row: ↑/↓, o.
      if (wide) {
        focusRow(id);
      }
    },
    [end, select, wide],
  );
  const navigate = useNavigate();
  const { country: open } = usePage();
  const chooseCountry = useCallback(
    (country: Country) => {
      end();
      // One step: the overview opens and the event closes, as in the apps; it frames itself. The
      // country already open opens afresh, once the event has closed.
      const again = open?.toUpperCase() === country.iso2;
      const go = async (): Promise<void> => {
        await navigate({ to: "/countries/$iso2", params: { iso2: country.iso2 }, search: {} });
        if (again) {
          reopenCountry();
        }
      };
      void go();
    },
    [end, navigate, open],
  );
  return (
    <SearchResults
      text={text}
      countries={countries}
      events={events}
      eventsLoaded={eventsLoaded}
      clustered={clustered}
      selectedId={selectedId}
      now={now}
      onEvent={chooseEvent}
      onCountry={chooseCountry}
    />
  );
}

export { SearchPanel };

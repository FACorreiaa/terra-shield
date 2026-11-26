import type { ReactElement } from "react";
import { useCallback } from "react";

import type { Country } from "@/domain/countries.ts";
import { mapBox } from "@/domain/countries.ts";
import { fitBox } from "@/features/map/fitBox.ts";
import { whenChromeRests } from "@/features/map/mapInsets.ts";
import { SearchResults } from "@/features/search/SearchResults.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import { useIsWide } from "./useBreakpoint.ts";
import { useSearchMatches } from "./useSearchMatches.ts";

/** A country is framed with this room around it, and never closer than a region's zoom. */
const COUNTRY_FIT = { breathingRoom: 32, maxZoom: 8 } as const;

/** Puts keyboard focus on the list's row for `id`, once the list is back in place of the results. */
function focusRow(id: string): void {
  requestAnimationFrame(() => {
    document.querySelector<HTMLElement>(`[data-event-id="${CSS.escape(id)}"] button`)?.focus();
  });
}

/**
 * Search's results, in place of the list while searching. Choosing an event selects it; choosing a
 * country clears the selection and frames the country on the map (its overview arrives with W5).
 */
function SearchPanel(): ReactElement {
  const { text, end } = useMapSearch();
  const { list, selectedId, select, now, map } = useLiveMap();
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
  const chooseCountry = useCallback(
    (country: Country) => {
      end();
      select(undefined);
      if (map === undefined) {
        return;
      }
      // A country across ±180° (Fiji, Russia) is framed by its events in view, as in the apps.
      const box = mapBox(
        country,
        list.filter((event) => event.countries.some((each) => each.iso2 === country.iso2)),
      );
      // Framed once the sheet or the inspector has settled, in the map they leave in view.
      whenChromeRests(() => {
        fitBox(map, box, COUNTRY_FIT);
      });
    },
    [end, select, map, list],
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

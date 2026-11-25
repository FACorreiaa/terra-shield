import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import type { View } from "@/domain/viewport.ts";
import { useEvents } from "@/features/events/useEvents.ts";
import { setQuery, useSavedFilters } from "@/features/filters/filterPrefs.ts";

import type { LiveMap } from "./liveMapContext.ts";
import { LiveMapContext } from "./liveMapContext.ts";
import { useSelectedEvent } from "./useSelectedEvent.ts";
import { useDisplayed, useShownEvents } from "./useShownEvents.ts";
import { useSourceHealth } from "./useSourceHealth.ts";
import { useUrlSelection } from "./useUrlSelection.ts";

const WHOLE_WORLD: View = { zoom: 1, bbox: "-180,-85,180,85" };
const CLOCK_TICK_MS = 30_000;

function useNow(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(new Date());
    }, CLOCK_TICK_MS);
    return () => {
      clearInterval(timer);
    };
  }, []);
  return now;
}

/**
 * Holds the live map's state above the routes and both shells, so neither a route change nor
 * crossing the 1024 px breakpoint resets the view or the filters. The selection is in the address.
 */
function LiveMapProvider({ children }: { children: ReactNode }): ReactElement {
  const [view, setView] = useState<View>(WHOLE_WORLD);
  const { selectedId, select, chosen, choose } = useUrlSelection();
  const [map, setMap] = useState<MapLibreMap>();
  const now = useNow();
  const { query } = useSavedFilters();
  const events = useEvents(view, query);
  const { shown, status, failure } = useShownEvents(events, now);
  const { list, clusters } = useDisplayed(shown);
  const { selected, detail, selectedShape } = useSelectedEvent(selectedId, list, chosen);
  const { sources, staleSources } = useSourceHealth(now);
  const value = useMemo<LiveMap>(
    () => ({
      view,
      setView,
      map,
      setMap,
      selectedId,
      select,
      choose,
      selected,
      detail,
      selectedShape,
      sources,
      staleSources,
      events,
      shown,
      list,
      clusters,
      query,
      setQuery,
      status,
      failure,
      now,
    }),
    [
      view,
      map,
      selectedId,
      select,
      choose,
      selected,
      detail,
      selectedShape,
      sources,
      staleSources,
      events,
      shown,
      list,
      clusters,
      query,
      status,
      failure,
      now,
    ],
  );
  return <LiveMapContext value={value}>{children}</LiveMapContext>;
}

export { LiveMapProvider };

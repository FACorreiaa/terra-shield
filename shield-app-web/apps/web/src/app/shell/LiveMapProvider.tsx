import type { ReactElement, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import type { View } from "@/domain/viewport.ts";

import { useLayerSelection } from "./layerEvents.ts";
import type { LiveMap } from "./liveMapContext.ts";
import { LiveMapContext } from "./liveMapContext.ts";
import { useArrivals } from "./useArrivals.ts";
import { useMapEvents } from "./useMapEvents.ts";
import { useMapFilters } from "./useMapFilters.ts";
import { SourceLookupContext, useSourceHealth } from "./useSourceHealth.ts";
import { useUrlSelection } from "./useUrlSelection.ts";

/**
 * The view before the map reports one: the world as the opening globe shows it (MapLibre zoom 1–2,
 * API zoom 2 at phone and desktop sizes), so the map's first report doesn't load everything again.
 */
const WHOLE_WORLD: View = { zoom: 2, bbox: "-180,-85,180,85" };
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
  const [map, setMap] = useState<LiveMap["map"]>();
  const now = useNow();
  const { query, setQuery, detections, conflicts } = useMapFilters(view);
  const arriving = useArrivals();
  const { events, shown, list, clusters, status, failure } = useMapEvents({
    view,
    query,
    selectedId,
    now,
    onArrival: arriving.note,
  });
  const { selected, detail, selectedShape, known, drawn } = useLayerSelection({ selectedId, chosen }, list, conflicts);
  const { sources, staleSources, lookup } = useSourceHealth(now);
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
      known,
      drawn,
      clusters,
      query,
      setQuery,
      status,
      failure,
      now,
      detections,
      conflicts,
      ...arriving,
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
      known,
      drawn,
      clusters,
      query,
      setQuery,
      status,
      failure,
      now,
      detections,
      conflicts,
      arriving,
    ],
  );
  return (
    <LiveMapContext value={value}>
      <SourceLookupContext value={lookup}>{children}</SourceLookupContext>
    </LiveMapContext>
  );
}

export { LiveMapProvider };

import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import { sortedForDisplay } from "@/domain/sort.ts";
import type { View } from "@/domain/viewport.ts";
import { useEvents } from "@/features/events/useEvents.ts";
import { setQuery, useSavedFilters } from "@/features/filters/filterPrefs.ts";

import type { LiveMap } from "./liveMapContext.ts";
import { LiveMapContext } from "./liveMapContext.ts";
import { useShownEvents } from "./useShownEvents.ts";

const NO_EVENTS: readonly HazardEvent[] = [];
const NO_CLUSTERS: readonly EventCluster[] = [];
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
 * crossing the 1024 px breakpoint resets the view, the filters or the selection. W5 moves the
 * selection into the URL (`/e/$id`) behind the same `select`.
 */
function LiveMapProvider({ children }: { children: ReactNode }): ReactElement {
  const [view, setView] = useState<View>(WHOLE_WORLD);
  const [selectedId, setSelectedId] = useState<string>();
  const [map, setMap] = useState<MapLibreMap>();
  const now = useNow();
  const { query } = useSavedFilters();
  const events = useEvents(view, query);
  const { shown, status, failure } = useShownEvents(events, now);
  // One order for the map's list and for ↑/↓: most severe, then newest, resolved last.
  const list = useMemo(() => (shown === undefined ? NO_EVENTS : sortedForDisplay(shown.events)), [shown]);
  const clusters = shown?.clusters ?? NO_CLUSTERS;
  const selected = useMemo(() => list.find((event) => event.id === selectedId), [list, selectedId]);
  const value = useMemo<LiveMap>(
    () => ({
      view,
      setView,
      map,
      setMap,
      selectedId,
      select: setSelectedId,
      selected,
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
    [view, map, selectedId, selected, events, shown, list, clusters, query, status, failure, now],
  );
  return <LiveMapContext value={value}>{children}</LiveMapContext>;
}

export { LiveMapProvider };

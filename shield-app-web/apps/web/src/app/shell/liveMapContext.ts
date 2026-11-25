import type { EventCluster, EventList, HazardEvent } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Map as MapLibreMap } from "maplibre-gl";
import { createContext, use } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import type { LiveStatus } from "@/domain/liveStatus.ts";
import type { View } from "@/domain/viewport.ts";

/** What every shell and section shares: the map's view, what is selected and the events in view. */
interface LiveMap {
  view: View;
  setView: (view: View) => void;
  /** The map, once created; it lives in the frame, below this provider. */
  map: MapLibreMap | undefined;
  setMap: (map: MapLibreMap | undefined) => void;
  selectedId: string | undefined;
  /** Selects an event, or clears the selection with `undefined`. */
  select: (id: string | undefined) => void;
  selected: HazardEvent | undefined;
  /** The request for the current view and filters: its state, and `refetch`. */
  events: UseQueryResult<EventList>;
  /** What the map and list show: the current response, else the last one that arrived. Never cleared. */
  shown: EventList | undefined;
  /** The shown events in display order: most severe, then newest, resolved last. */
  list: readonly HazardEvent[];
  clusters: readonly EventCluster[];
  /** The filters, kept across visits; setting them reloads the map behind the sheet. */
  query: EventQuery;
  setQuery: (query: EventQuery) => void;
  /** The capsule's words and colours. */
  status: LiveStatus;
  /** Why the last request failed, in words for people, while online; undefined otherwise. */
  failure: string | undefined;
  /** Ticks every 30 s so relative times ("12 min ago") stay true. */
  now: Date;
}

const LiveMapContext = createContext<LiveMap | undefined>(undefined);

function useLiveMap(): LiveMap {
  const value = use(LiveMapContext);
  if (value === undefined) {
    throw new Error("useLiveMap needs a LiveMapProvider above it");
  }
  return value;
}

export type { LiveMap };
export { LiveMapContext, useLiveMap };

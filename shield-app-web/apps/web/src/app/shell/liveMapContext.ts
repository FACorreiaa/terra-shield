import type { EventCluster, EventList, HazardEvent } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { createContext, use } from "react";

import type { View } from "@/domain/viewport.ts";

/** What every shell and section shares: the map's view, what is selected and the events in view. */
interface LiveMap {
  view: View;
  setView: (view: View) => void;
  selectedId: string | undefined;
  /** Selects an event, or clears the selection with `undefined`. */
  select: (id: string | undefined) => void;
  selected: HazardEvent | undefined;
  events: UseQueryResult<EventList>;
  list: readonly HazardEvent[];
  clusters: readonly EventCluster[];
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

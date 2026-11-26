import type { EventCluster, EventList, HazardEvent, Source } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Map as MapLibreMap } from "maplibre-gl";
import { createContext, use } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import type { EventShape } from "@/domain/geo.ts";
import type { LiveStatus } from "@/domain/liveStatus.ts";
import type { View } from "@/domain/viewport.ts";
import type { DetailState } from "@/features/detail/detailState.ts";

import type { MapFilters } from "./useMapFilters.ts";

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
  /** Selects an event from outside the map's list (a country's), which the map then draws too. */
  choose: (event: HazardEvent) => void;
  /** The selected aircraft's address, while one is: never with an event (live flights, ADR 28). */
  aircraftHex: string | undefined;
  /** Selects an aircraft, clearing the event, or clears it with `undefined`. */
  selectAircraft: (hex: string | undefined) => void;
  /** An event or an aircraft is selected: the inspector and the sheet show its detail. */
  inspecting: boolean;
  /** The selected event: the map's copy, else the detail's, else as last seen once gone. */
  selected: HazardEvent | undefined;
  /** The selection's detail as it loads (DESIGN.md §10.3); undefined with nothing selected. */
  detail: DetailState | undefined;
  /** The selection's area, once its detail has loaded; drawn under the markers. */
  selectedShape: EventShape | undefined;
  /** From `/v1/sources`; undefined until it loads. */
  sources: readonly Source[] | undefined;
  /** Sources that have missed three polls: their events fade. */
  staleSources: ReadonlySet<string>;
  /** The request for the current view and filters: its state, and `refetch`. */
  events: UseQueryResult<EventList>;
  /** What the map and list show: the current response, else the last one that arrived. Never cleared. */
  shown: EventList | undefined;
  /** The shown events in display order: most severe, then newest, resolved last. */
  list: readonly HazardEvent[];
  /** The hazards, then the conflicts layer's: what selects, and what ↑ ↓ step through. */
  known: readonly HazardEvent[];
  /** What the map draws: `known`, and the selection when it is in neither. */
  drawn: readonly HazardEvent[];
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
  /** Satellite heat detections for the view, while the layer is on (W7). */
  detections: MapFilters["detections"];
  /** Armed conflicts for the view, while that layer is on (W7, ADR 27): never in `list` or `clusters`. */
  conflicts: MapFilters["conflicts"];
  /** Live arrivals in view the map hasn't animated in yet (W6). */
  arrivals: ReadonlySet<string>;
  /** The map has begun animating an arrival. */
  started: (id: string) => void;
  /** The map has animated an arrival in. */
  arrived: (id: string) => void;
  /** The newest severe or extreme live arrival in view, for 6 s or until dismissed. */
  toast: HazardEvent | undefined;
  dismissToast: () => void;
  /** Pointer or focus on the toast stops its clock. */
  holdToast: (held: boolean) => void;
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

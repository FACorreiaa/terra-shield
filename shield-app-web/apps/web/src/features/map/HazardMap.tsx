import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement, ReactNode } from "react";
import { useEffect, useEffectEvent, useRef } from "react";

import type { EventShape } from "@/domain/geo.ts";
import type { View } from "@/domain/viewport.ts";

import type { ArrivalCallbacks } from "./ArrivalPin.tsx";
import type { Projection } from "./basemapStyle.ts";
import { MapOverlays } from "./MapOverlays.tsx";
import type { HazardLayerData } from "./useHazardLayers.ts";
import { useHazardLayers } from "./useHazardLayers.ts";
import { useHazardMap } from "./useHazardMap.ts";

interface HazardMapProps {
  /** Hazards and, while that layer is on, armed conflicts: one marker layer, conflicts under hazards. */
  events: readonly HazardEvent[];
  clusters: readonly EventCluster[];
  /** Armed-conflict clusters (W7, ADR 27); none while the layer is off. */
  conflictClusters?: readonly EventCluster[] | undefined;
  /** Sources whose data is stale: their markers fade and get a clock. */
  staleSources: ReadonlySet<string>;
  selectedId: string | undefined;
  /** The selected event's footprint, once its detail has loaded. */
  selectedShape: EventShape | undefined;
  projection: Projection;
  onViewChange: (view: View) => void;
  onSelect: (id: string) => void;
  /** The live map, for controls outside it (zoom, fit world); undefined while it is loading. */
  onMap?: (map: MapLibreMap | undefined) => void;
  /** Live arrivals to animate in (W6), and what the map reports as each plays. */
  arrivals?: ReadonlySet<string> | undefined;
  arrival?: ArrivalCallbacks | undefined;
  /** Satellite heat detections (W7); none while the layer is off. */
  detections?: HazardLayerData["detections"];
  /** More canvas layers drawn with the map once it exists (live flights), given the style's epoch. */
  layers?: ((map: MapLibreMap, styleEpoch: number) => ReactNode) | undefined;
}

const NO_ARRIVALS: ReadonlySet<string> = new Set();
const NO_CLUSTERS: readonly EventCluster[] = [];

/**
 * The map canvas (DESIGN.md §9): OpenFreeMap basemap, the selected event's area, satellite heat
 * detections, point markers with the selection ring, server clusters as buttons and the extreme pulse.
 */
function HazardMap(props: HazardMapProps): ReactElement {
  const { events, clusters, staleSources, projection, onViewChange, onMap, arrivals = NO_ARRIVALS, arrival } = props;
  const { conflictClusters = NO_CLUSTERS, layers } = props;
  const container = useRef<HTMLDivElement>(null);
  const { map, styleEpoch } = useHazardMap(container, { projection, onViewChange });
  const mapChanged = useEffectEvent((next: MapLibreMap | undefined) => onMap?.(next));

  useEffect(() => {
    mapChanged(map);
  }, [map]);
  useHazardLayers(map, styleEpoch, props);

  // MapLibre's own CSS makes its container `position: relative`, so positioning lives on a wrapper.
  return (
    <div className="absolute inset-0" data-testid="hazard-map">
      <div ref={container} className="size-full" />
      {map && (
        <MapOverlays
          map={map}
          clusters={clusters}
          conflictClusters={conflictClusters}
          events={events}
          staleSources={staleSources}
          arriving={arrivals}
          arrival={arrival}
        />
      )}
      {map && layers?.(map, styleEpoch)}
    </div>
  );
}

export type { HazardMapProps };
export { HazardMap };

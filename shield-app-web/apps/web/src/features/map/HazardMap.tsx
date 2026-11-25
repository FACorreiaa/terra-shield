import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useEffect, useEffectEvent, useRef } from "react";

import type { EventShape } from "@/domain/geo.ts";
import type { View } from "@/domain/viewport.ts";

import type { Projection } from "./basemapStyle.ts";
import { MapOverlays } from "./MapOverlays.tsx";
import { useHazardLayers } from "./useHazardLayers.ts";
import { useHazardMap } from "./useHazardMap.ts";

interface HazardMapProps {
  events: readonly HazardEvent[];
  clusters: readonly EventCluster[];
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
}

/**
 * The map canvas (DESIGN.md §9): OpenFreeMap basemap, the selected event's area, point markers
 * with the selection ring, server clusters as buttons and the extreme pulse.
 */
function HazardMap(props: HazardMapProps): ReactElement {
  const { events, clusters, staleSources, projection, onViewChange, onMap } = props;
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
      {map && <MapOverlays map={map} clusters={clusters} events={events} staleSources={staleSources} />}
    </div>
  );
}

export type { HazardMapProps };
export { HazardMap };

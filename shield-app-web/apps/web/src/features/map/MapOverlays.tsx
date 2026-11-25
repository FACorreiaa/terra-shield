import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { ClusterMarker } from "./ClusterMarker.tsx";
import { ExtremeHalo } from "./ExtremeHalo.tsx";
import { insetsPlus } from "./mapInsets.ts";
import { MapMarker } from "./MapMarker.tsx";
import { markerSize } from "./markerArt.ts";

const CLUSTER_FIT = { breathingRoom: 48, maxZoom: 7 } as const;
const FULL_TURN = 360;

/** Zooms to a cluster's extent; a box across ±180° is unwrapped so the camera takes the short way. */
function fitCluster(map: MapLibreMap, cluster: EventCluster): void {
  const [west, south, east, north] = cluster.bbox;
  if (west === undefined || south === undefined || east === undefined || north === undefined) {
    return;
  }
  map.fitBounds([west, south, east < west ? east + FULL_TURN : east, north], {
    padding: insetsPlus(CLUSTER_FIT.breathingRoom),
    maxZoom: CLUSTER_FIT.maxZoom,
  });
}

function ClusterPin({ map, cluster }: { map: MapLibreMap; cluster: EventCluster }): ReactElement {
  const activate = useCallback(() => {
    fitCluster(map, cluster);
  }, [map, cluster]);
  return (
    <MapMarker map={map} longitude={cluster.centroid.longitude} latitude={cluster.centroid.latitude}>
      <ClusterMarker cluster={cluster} onActivate={activate} />
    </MapMarker>
  );
}

interface MapOverlaysProps {
  map: MapLibreMap;
  clusters: readonly EventCluster[];
  events: readonly HazardEvent[];
  staleSources: ReadonlySet<string>;
}

/** The DOM layer over the canvas: the extreme pulse under server clusters (real buttons). */
function MapOverlays({ map, clusters, events, staleSources }: MapOverlaysProps): ReactElement {
  const pulsing = events.filter(
    (event) =>
      event.severity === "extreme" &&
      event.status !== "resolved" &&
      event.kind !== "detection" &&
      !staleSources.has(event.source),
  );
  return (
    <>
      {pulsing.map((event) => (
        <MapMarker key={event.id} map={map} longitude={event.centroid.longitude} latitude={event.centroid.latitude}>
          <ExtremeHalo size={markerSize("extreme")} />
        </MapMarker>
      ))}
      {clusters.map((cluster) => (
        <ClusterPin
          key={`${cluster.centroid.longitude},${cluster.centroid.latitude},${cluster.count}`}
          map={map}
          cluster={cluster}
        />
      ))}
    </>
  );
}

export { MapOverlays };

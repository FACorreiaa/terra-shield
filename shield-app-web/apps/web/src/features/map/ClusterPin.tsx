import type { EventCluster } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { geoBox } from "@/domain/geo.ts";

import { ClusterMarker } from "./ClusterMarker.tsx";
import { fitBox } from "./fitBox.ts";
import { MapMarker } from "./MapMarker.tsx";

const CLUSTER_FIT = { breathingRoom: 48, maxZoom: 7 } as const;

/** Zooms to a cluster's extent. */
function fitCluster(map: MapLibreMap, cluster: EventCluster): void {
  const box = geoBox(cluster.bbox);
  if (box !== undefined) {
    fitBox(map, box, CLUSTER_FIT);
  }
}

interface ClusterPinProps {
  map: MapLibreMap;
  cluster: EventCluster;
  conflict?: boolean;
}

function ClusterPin({ map, cluster, conflict = false }: ClusterPinProps): ReactElement {
  const activate = useCallback(() => {
    fitCluster(map, cluster);
  }, [map, cluster]);
  return (
    <MapMarker map={map} longitude={cluster.centroid.longitude} latitude={cluster.centroid.latitude}>
      <ClusterMarker cluster={cluster} onActivate={activate} conflict={conflict} />
    </MapMarker>
  );
}

export { ClusterPin };

import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";

import { CONFLICT } from "@/domain/taxonomy.ts";

import type { ArrivalCallbacks } from "./ArrivalPin.tsx";
import { ArrivalPins } from "./ArrivalPin.tsx";
import { ClusterPin } from "./ClusterPin.tsx";
import { ExtremeHalo } from "./ExtremeHalo.tsx";
import { MapMarker } from "./MapMarker.tsx";
import { markerSize } from "./markerArt.ts";

function clusterKey(cluster: EventCluster): string {
  return `${cluster.centroid.longitude},${cluster.centroid.latitude},${cluster.count}`;
}

interface MapOverlaysProps {
  map: MapLibreMap;
  clusters: readonly EventCluster[];
  /** Armed-conflict clusters (ADR 27): their own, never merged with hazards'. */
  conflictClusters: readonly EventCluster[];
  events: readonly HazardEvent[];
  staleSources: ReadonlySet<string>;
  /** Live arrivals not yet animated in. */
  arriving: ReadonlySet<string>;
  arrival: ArrivalCallbacks | undefined;
}

/** The DOM layer over the canvas: the extreme pulse and live arrivals under server clusters (real buttons). */
function MapOverlays(props: MapOverlaysProps): ReactElement {
  const { map, clusters, conflictClusters, events, staleSources, arriving, arrival } = props;
  const pulsing = events.filter(
    (event) =>
      event.severity === "extreme" &&
      event.status !== "resolved" &&
      event.kind !== "detection" &&
      // A conflict is never stale (a monthly estimate); an extreme one pulses as in the apps.
      (event.type === CONFLICT || !staleSources.has(event.source)),
  );
  return (
    <>
      {pulsing.map((event) => (
        <MapMarker key={event.id} map={map} longitude={event.centroid.longitude} latitude={event.centroid.latitude}>
          {/* An arriving extreme marker's ring comes in with it, not ahead of it. */}
          <div className={arriving.has(event.id) ? "arrive" : undefined}>
            <ExtremeHalo size={markerSize("extreme")} />
          </div>
        </MapMarker>
      ))}
      {arrival === undefined ? undefined : (
        <ArrivalPins map={map} events={events} staleSources={staleSources} arriving={arriving} arrival={arrival} />
      )}
      {/* Before the hazards' clusters, so a hazard cluster draws over a conflict one (ADR 27: hazards first). */}
      {conflictClusters.map((cluster) => (
        <ClusterPin key={`conflict/${clusterKey(cluster)}`} map={map} cluster={cluster} conflict />
      ))}
      {clusters.map((cluster) => (
        <ClusterPin key={clusterKey(cluster)} map={map} cluster={cluster} />
      ))}
    </>
  );
}

export { MapOverlays };

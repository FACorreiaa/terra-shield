import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { GeoJSONSource } from "maplibre-gl";
import { Map as MapLibreMap, Marker, NavigationControl } from "maplibre-gl";
import type { ReactElement, RefObject } from "react";
import { useEffect, useEffectEvent, useRef, useState } from "react";

import { severityTokens } from "@/design/tokens.gen.ts";

import { basemapStyle } from "./basemap.ts";
import { cssVar, eventFeatures, eventLayer } from "./eventLayer.ts";
import { addMarkerImages } from "./markerImages.ts";

interface MapView {
  zoom: number;
  bbox: string;
}

interface HazardMapProps {
  events: readonly HazardEvent[];
  clusters: readonly EventCluster[];
  onViewChange: (view: MapView) => void;
  onSelect: (id: string) => void;
}

const INITIAL_LONGITUDE = 10;
const INITIAL_LATITUDE = 25;
const INITIAL_CENTER: [number, number] = [INITIAL_LONGITUDE, INITIAL_LATITUDE];
const INITIAL_ZOOM = 1.4;
const BBOX_DECIMALS = 100;
const CLUSTER_FIT_PADDING = 48;
const CLUSTER_FIT_MAX_ZOOM = 7;

function round(value: number): number {
  return Math.round(value * BBOX_DECIMALS) / BBOX_DECIMALS;
}

function viewOf(map: MapLibreMap): MapView {
  const bounds = map.getBounds();
  return {
    zoom: Math.round(map.getZoom()),
    bbox: [bounds.getWest(), bounds.getSouth(), bounds.getEast(), bounds.getNorth()]
      .map((value) => round(value))
      .join(","),
  };
}

/** Creates the map once; hands it back after the style and marker images have loaded. */
function useMap(
  container: RefObject<HTMLDivElement | null>,
  onViewChange: (view: MapView) => void,
  onSelect: (id: string) => void,
): MapLibreMap | undefined {
  const [ready, setReady] = useState<MapLibreMap>();
  const viewChanged = useEffectEvent(onViewChange);
  const selected = useEffectEvent(onSelect);

  useEffect(() => {
    const element = container.current;
    if (element === null) {
      return undefined;
    }
    const map = new MapLibreMap({
      container: element,
      style: basemapStyle({
        canvas: cssVar("--color-canvas"),
        panel: cssVar("--color-panel"),
        hairline: cssVar("--color-hairline"),
      }),
      center: INITIAL_CENTER,
      zoom: INITIAL_ZOOM,
      renderWorldCopies: false,
      attributionControl: { compact: true, customAttribution: "Natural Earth" },
    });
    map.addControl(new NavigationControl({ showCompass: false }), "bottom-right");
    map.on("load", () => {
      addMarkerImages(map);
      map.addSource("events", { type: "geojson", data: eventFeatures([]) });
      map.addLayer(eventLayer());
      map.on("click", "events", (event) => {
        const id: unknown = event.features?.[0]?.properties["id"];
        if (typeof id === "string") {
          selected(id);
        }
      });
      setReady(map);
      viewChanged(viewOf(map));
    });
    map.on("moveend", () => {
      viewChanged(viewOf(map));
    });
    return () => {
      setReady(undefined);
      map.remove();
    };
  }, [container]);

  return ready;
}

function clusterMarker(map: MapLibreMap, cluster: EventCluster): Marker {
  const el = document.createElement("button");
  el.type = "button";
  el.className = "hazard-cluster";
  el.dataset["severity"] = cluster.maxSeverity;
  el.textContent = String(cluster.count);
  el.setAttribute("aria-label", `${cluster.count} events, up to ${severityTokens[cluster.maxSeverity].label}`);
  el.addEventListener("click", () => {
    const [west, south, east, north] = cluster.bbox;
    if (west !== undefined && south !== undefined && east !== undefined && north !== undefined) {
      map.fitBounds([west, south, east, north], { padding: CLUSTER_FIT_PADDING, maxZoom: CLUSTER_FIT_MAX_ZOOM });
    }
  });
  return new Marker({ element: el }).setLngLat([cluster.centroid.longitude, cluster.centroid.latitude]).addTo(map);
}

/** The map canvas: Natural Earth basemap, events as severity shapes, server clusters as counts. */
function HazardMap({ events, clusters, onViewChange, onSelect }: HazardMapProps): ReactElement {
  const container = useRef<HTMLDivElement>(null);
  const map = useMap(container, onViewChange, onSelect);

  useEffect(() => {
    void map?.getSource<GeoJSONSource>("events")?.setData(eventFeatures(events));
  }, [map, events]);

  useEffect(() => {
    if (map === undefined) {
      return undefined;
    }
    const markers = clusters.map((cluster) => clusterMarker(map, cluster));
    return () => {
      for (const marker of markers) {
        marker.remove();
      }
    };
  }, [map, clusters]);

  return <div ref={container} className="absolute inset-0 [&_canvas]:cursor-grab" data-testid="hazard-map" />;
}

export type { MapView };
export { HazardMap };

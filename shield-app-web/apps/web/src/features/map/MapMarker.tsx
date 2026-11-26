import type { Map as MapLibreMap } from "maplibre-gl";
import { Marker } from "maplibre-gl";
import type { ReactElement, ReactNode } from "react";
import { useEffect, useMemo } from "react";
import { createPortal } from "react-dom";

interface MapMarkerProps {
  map: MapLibreMap;
  longitude: number;
  latitude: number;
  children: ReactNode;
}

/**
 * React content pinned to a map coordinate: a MapLibre `Marker` whose element React renders into,
 * so markers that must be real buttons (clusters) or animate (the extreme pulse) stay React.
 */
function MapMarker({ map, longitude, latitude, children }: MapMarkerProps): ReactElement {
  const element = useMemo(() => document.createElement("div"), []);
  const marker = useMemo(() => new Marker({ element, opacityWhenCovered: "0" }), [element]);

  // Declared first so the position is set before `addTo` draws the marker.
  useEffect(() => {
    marker.setLngLat([longitude, latitude]);
  }, [marker, longitude, latitude]);

  useEffect(() => {
    marker.addTo(map);
    return () => {
      marker.remove();
    };
  }, [map, marker]);

  return createPortal(children, element);
}

export { MapMarker };

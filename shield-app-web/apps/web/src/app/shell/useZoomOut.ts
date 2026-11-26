import { useCallback } from "react";

import { mapPrefs } from "@/features/map/mapPrefs.ts";
import { fitWorld } from "@/features/map/useHazardMap.ts";

import { useLiveMap } from "./liveMapContext.ts";

/** Shows the whole world, in the projection the person chose. */
function useZoomOut(): () => void {
  const { map } = useLiveMap();
  return useCallback(() => {
    if (map !== undefined) {
      fitWorld(map, mapPrefs().projection);
    }
  }, [map]);
}

export { useZoomOut };

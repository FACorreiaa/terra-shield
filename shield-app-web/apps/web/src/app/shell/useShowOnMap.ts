import { use, useCallback, useEffect, useRef } from "react";

import { frame } from "@/domain/geo.ts";
import { fitBox } from "@/features/map/fitBox.ts";
import { whenChromeRests } from "@/features/map/mapInsets.ts";

import { useLiveMap } from "./liveMapContext.ts";
import { SheetDetentSetter } from "./sheetDetents.ts";

/** An event is framed with room around it, never closer than a town's zoom. */
const SHOW_FIT = { breathingRoom: 32, maxZoom: 9 } as const;

/** Frames the selection on the main map, its area when loaded, after the sheet has lowered to half height. */
function useShowOnMap(): () => void {
  const { selected, selectedShape, map } = useLiveMap();
  const setDetent = use(SheetDetentSetter);
  return useCallback(() => {
    if (selected === undefined || map === undefined) {
      return;
    }
    setDetent?.("medium");
    const box = frame(selectedShape ?? { kind: "point", point: selected.centroid }, selected.centroid);
    whenChromeRests(() => {
      fitBox(map, box, SHOW_FIT);
    });
  }, [selected, selectedShape, map, setDetent]);
}

/**
 * A page opened on an event (a shared link, a reload) shows it on the map once it is known; the
 * whole world would hide it in a cluster. Selections made afterwards leave the camera alone.
 */
function useShowOnArrival(): void {
  const { selectedId, selected, detail, map } = useLiveMap();
  const arrival = useRef(selectedId);
  const show = useShowOnMap();
  useEffect(() => {
    if (selectedId !== arrival.current) {
      // Closed or switched before the event was known: a later selection leaves the camera alone.
      arrival.current = undefined;
    }
    // Waits for the detail, so an area is framed rather than its centroid.
    const known = selected?.id === arrival.current && detail !== undefined && detail.kind !== "loading";
    if (arrival.current === undefined || !known || map === undefined) {
      return;
    }
    arrival.current = undefined;
    show();
  }, [selectedId, selected, detail, map, show]);
}

export { useShowOnArrival, useShowOnMap };

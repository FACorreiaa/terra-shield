import type { RefObject } from "react";
import { useCallback } from "react";

import { setMapInsets } from "@/features/map/mapInsets.ts";

/** The space the shell's chrome takes over the map, per side, in px. */
interface Insets {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

/** DESIGN.md §10: the sidebar is 220–260 px (248 here) and the inspector 320–380. */
const SIDEBAR_WIDTH_CLASS = "w-[248px]";
const MAP_LEFT_CLASS = "left-[248px]";
/** Matches the inspector's `w-[352px]`. */
const INSPECTOR_WIDTH = 352;
/** Floating chrome keeps this much clear of the map's edge. */
const MAP_GAP = 12;
/** The phone's status capsule: 44 px tall under a 12 px margin. */
const CAPSULE_INSET = 56;

/**
 * Hands what the chrome covers to the camera, and to the map's own controls as CSS variables on the
 * frame, written directly so a resting sheet never re-renders the app.
 */
function useInsets(frame: RefObject<HTMLElement | null>): (insets: Insets) => void {
  return useCallback(
    (next: Insets) => {
      setMapInsets(next);
      for (const side of ["top", "right", "bottom"] as const) {
        frame.current?.style.setProperty(`--map-inset-${side}`, `${next[side]}px`);
      }
    },
    [frame],
  );
}

export type { Insets };
export { CAPSULE_INSET, INSPECTOR_WIDTH, MAP_GAP, MAP_LEFT_CLASS, SIDEBAR_WIDTH_CLASS, useInsets };

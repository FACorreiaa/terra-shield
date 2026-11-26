import type { RefObject } from "react";
import { useCallback, useLayoutEffect, useRef, useState } from "react";

import type { Detent, Heights } from "./sheetDetents.ts";
import { detentAt } from "./sheetDetents.ts";
import type { SheetMotion } from "./useSheetMotion.ts";

interface PlacementOptions {
  motion: SheetMotion;
  heights: Heights;
  /** False until the sheet's container has a size. */
  measured: boolean;
  detent: Detent;
  onDetentChange: (detent: Detent) => void;
  onRest: (height: number, heights: Heights) => void;
}

interface Placement {
  /** The height the sheet last came to rest at; the scroll area is sized to it. */
  rested: number;
  /** Throws the sheet from a released drag to `height` with the release `velocity`. */
  settle: (height: number, velocity: number) => void;
}

/** The height of `element` (or its parent), kept current by a ResizeObserver. */
function useHeight(element: RefObject<HTMLElement | null>, parent = false): number {
  const [height, setHeight] = useState(0);
  useLayoutEffect(() => {
    const target = parent ? element.current?.parentElement : element.current;
    if (target === null || target === undefined) {
      return undefined;
    }
    // A ResizeObserver reports once on `observe`, before the first paint.
    const observer = new ResizeObserver(() => {
      setHeight(target.clientHeight);
    });
    observer.observe(target);
    return () => {
      observer.disconnect();
    };
  }, [element, parent]);
  return height;
}

/**
 * Keeps the sheet on its detent: placed without motion the first time, sprung there when the detent
 * changes from outside (a selection, the handle, a key), placed again when the screen resizes. A
 * drag's own release is already springing, so its detent change does not restart the motion.
 */
function useSheetPlacement(options: PlacementOptions): Placement {
  const { motion, heights, measured, detent, onDetentChange, onRest } = options;
  const [rested, setRested] = useState(0);
  const placed = useRef(false);
  /** The detent a released drag is already springing to. */
  const heading = useRef<Detent>(undefined);

  const rest = useCallback(
    (height: number) => {
      setRested(height);
      onRest(height, heights);
    },
    [onRest, heights],
  );

  const settle = useCallback(
    (height: number, velocity: number) => {
      const target = detentAt(height, heights);
      heading.current = target === detent ? undefined : target;
      motion.springTo(height, velocity, () => {
        rest(height);
      });
      onDetentChange(target);
    },
    [heights, detent, motion, rest, onDetentChange],
  );

  useLayoutEffect(() => {
    if (!measured) {
      return;
    }
    const height = heights[detent];
    if (heading.current === detent) {
      heading.current = undefined;
    } else if (placed.current) {
      motion.springTo(height, 0, () => {
        rest(height);
      });
    } else {
      placed.current = true;
      motion.place(height);
      rest(height);
    }
  }, [measured, heights, detent, motion, rest]);

  return { rested, settle };
}

export { useHeight, useSheetPlacement };

import type { PointerEvent as ReactPointerEvent } from "react";
import { useCallback, useRef } from "react";

import type { Heights } from "./sheetDetents.ts";
import { rubberBand, snapTarget } from "./sheetPhysics.ts";
import type { SheetMotion } from "./useSheetMotion.ts";

/** A press that moves less than this is a tap, not a drag (Apple's hysteresis). */
const DRAG_THRESHOLD_PX = 6;
/** Release velocity is measured over the last stretch of the drag. */
const VELOCITY_WINDOW_MS = 80;
const MS_PER_SECOND = 1000;
/** How many recent pointer samples are kept for the release velocity. */
const VELOCITY_SAMPLES = 8;
/** Presses on these keep their own meaning (a click, typing) instead of starting a drag. */
const INTERACTIVE = "input, textarea, select, a, button:not([data-sheet-handle])";

interface Sample {
  height: number;
  time: number;
}

interface Gesture {
  pointerId: number;
  startY: number;
  startHeight: number;
  dragging: boolean;
  samples: Sample[];
}

interface SheetDrag {
  onPointerDown: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerMove: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerUp: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerCancel: (event: ReactPointerEvent<HTMLElement>) => void;
}

function releaseVelocity(samples: readonly Sample[]): number {
  const last = samples.at(-1);
  const first = samples.find((sample) => last !== undefined && last.time - sample.time <= VELOCITY_WINDOW_MS);
  if (last === undefined || first === undefined || last.time === first.time) {
    return 0;
  }
  return ((last.height - first.height) / (last.time - first.time)) * MS_PER_SECOND;
}

/** The detent a release at `height` and `velocity` is heading for. */
function landing(height: number, velocity: number, heights: Heights): number {
  return snapTarget(height, velocity, [heights.peek, heights.medium, heights.large]);
}

/**
 * Drags the sheet by its header, 1:1 with the finger from where it was grabbed, softly past the
 * top and bottom detents, then throws it to the detent the release is heading for (DESIGN.md §10).
 * Grabbing a moving sheet stops it where it is.
 */
function useSheetDrag(
  motion: SheetMotion,
  heights: Heights,
  { settle, tap }: { settle: (height: number, velocity: number) => void; tap: () => void },
): SheetDrag {
  const gesture = useRef<Gesture>(undefined);

  const onPointerDown = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      if (event.button !== 0 || (event.target instanceof Element && event.target.closest(INTERACTIVE) !== null)) {
        return;
      }
      motion.stop();
      // Captured at once: the first move can already be outside the header.
      event.currentTarget.setPointerCapture(event.pointerId);
      gesture.current = {
        pointerId: event.pointerId,
        startY: event.clientY,
        startHeight: motion.position.current,
        dragging: false,
        samples: [],
      };
    },
    [motion],
  );

  const onPointerMove = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      const { current } = gesture;
      if (current?.pointerId !== event.pointerId) {
        return;
      }
      const travel = current.startY - event.clientY;
      if (!current.dragging && Math.abs(travel) >= DRAG_THRESHOLD_PX) {
        current.dragging = true;
      }
      if (current.dragging) {
        const height = rubberBand(
          current.startHeight + travel,
          { min: heights.peek, max: heights.large },
          heights.large,
        );
        motion.place(height);
        current.samples = [...current.samples.slice(-VELOCITY_SAMPLES), { height, time: event.timeStamp }];
      }
    },
    [motion, heights],
  );

  /**
   * Release or cancel: a drag is thrown to its detent (a cancelled one, where the browser took the
   * pointer, without the throw); a press on the header that never moved is a tap, so the whole
   * header, not only the 24 px grabber, is the tap target.
   */
  const onPointerUp = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      const { current } = gesture;
      if (current?.pointerId !== event.pointerId) {
        return;
      }
      gesture.current = undefined;
      const cancelled = event.type === "pointercancel";
      if (current.dragging) {
        const velocity = cancelled ? 0 : releaseVelocity(current.samples);
        settle(landing(motion.position.current, velocity, heights), velocity);
      } else if (!cancelled) {
        tap();
      }
    },
    [motion, heights, settle, tap],
  );

  return { onPointerDown, onPointerMove, onPointerUp, onPointerCancel: onPointerUp };
}

export { useSheetDrag };

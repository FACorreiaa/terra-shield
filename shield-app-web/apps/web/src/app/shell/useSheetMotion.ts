import type { RefObject } from "react";
import { useCallback, useEffect, useMemo, useRef } from "react";

import type { SpringState } from "./sheetPhysics.ts";
import { atRest, springStep } from "./sheetPhysics.ts";

/** A long frame (a tab coming back) is stepped as this much, so the spring never jumps. */
const FRAME = { longestMs: 33 } as const;
const MS_PER_SECOND = 1000;
const REDUCED_MOTION = "(prefers-reduced-motion: reduce)";

interface SheetMotion {
  /** The sheet's live visible height, which a drag or interruption starts from. */
  position: RefObject<number>;
  /** Shows the sheet at `height` now, cancelling any motion. */
  place: (height: number) => void;
  /** Springs to `height` from where the sheet is, carrying `velocity` (px/s, up is positive). */
  springTo: (height: number, velocity: number, onRest: () => void) => void;
  /** Stops any motion where it is (a finger caught the sheet). */
  stop: () => void;
}

/**
 * Moves the sheet with a spring on `requestAnimationFrame`, writing its transform directly so a
 * drag never re-renders React. `largest` is the sheet's own height: it is laid out at the large
 * detent and translated down to show less. Under reduced motion it jumps (DESIGN.md §14); the CSS
 * rule cannot reach a JavaScript spring, so this checks the media query itself.
 */
function useSheetMotion(sheet: RefObject<HTMLElement | null>, largest: number): SheetMotion {
  const position = useRef(0);
  const frame = useRef(0);

  const draw = useCallback(
    (height: number) => {
      position.current = height;
      sheet.current?.style.setProperty("transform", `translate3d(0, ${largest - height}px, 0)`);
    },
    [sheet, largest],
  );

  const stop = useCallback(() => {
    cancelAnimationFrame(frame.current);
    frame.current = 0;
  }, []);

  const place = useCallback(
    (height: number) => {
      stop();
      draw(height);
    },
    [stop, draw],
  );

  const springTo = useCallback(
    (height: number, velocity: number, onRest: () => void) => {
      stop();
      if (globalThis.matchMedia(REDUCED_MOTION).matches) {
        draw(height);
        onRest();
        return;
      }
      let state: SpringState = { position: position.current, velocity };
      let last = performance.now();
      const tick = (time: number): void => {
        const seconds = Math.min(FRAME.longestMs, time - last) / MS_PER_SECOND;
        last = time;
        state = springStep(state, height, seconds);
        if (atRest(state, height)) {
          draw(height);
          frame.current = 0;
          onRest();
          return;
        }
        draw(state.position);
        frame.current = requestAnimationFrame(tick);
      };
      frame.current = requestAnimationFrame(tick);
    },
    [stop, draw],
  );

  useEffect(() => stop, [stop]);

  return useMemo(() => ({ position, place, springTo, stop }), [place, springTo, stop]);
}

export type { SheetMotion };
export { useSheetMotion };

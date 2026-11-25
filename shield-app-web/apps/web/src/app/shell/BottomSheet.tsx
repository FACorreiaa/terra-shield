import type { ComponentType, CSSProperties, KeyboardEvent, MouseEvent, ReactElement, ReactNode } from "react";
import { useCallback, useMemo, useRef } from "react";

import type { Detent, Heights } from "./sheetDetents.ts";
import { detentAt, detentHeights, DETENTS, nextOnPress } from "./sheetDetents.ts";
import { stepDetent } from "./sheetPhysics.ts";
import { useSheetDrag } from "./useSheetDrag.ts";
import { useSheetMotion } from "./useSheetMotion.ts";
import { useHeight, useSheetPlacement } from "./useSheetPlacement.ts";

interface BottomSheetProps {
  /** Names the sheet for assistive tech. */
  label: string;
  /** Always visible, even at peek: the sheet's header (search and the compact strip, W4). */
  Header: ComponentType;
  children: ReactNode;
  detent: Detent;
  onDetentChange: (detent: Detent) => void;
  /** The height the sheet covers once it comes to rest, for the map's insets. */
  onRest: (height: number, heights: Heights) => void;
}

interface Handle {
  advance: () => void;
  press: (event: MouseEvent) => void;
  keys: (event: KeyboardEvent) => void;
}

/** Spoken with the handle: the sheet's current size. */
const DETENT_WORDS: Readonly<Record<Detent, string>> = {
  peek: "collapsed",
  medium: "half height",
  large: "full height",
};
const KEY_STEPS: Readonly<Partial<Record<string, 1 | -1>>> = { ArrowUp: 1, ArrowDown: -1 };

/**
 * The handle: a tap or Enter walks the detents (peek → medium → large → peek), ↑/↓ step one at a
 * time. Pointer taps come from the drag (the pointer is captured, so the button's click is the
 * keyboard's alone).
 */
function useHandle({
  detent,
  heights,
  onDetentChange,
}: {
  detent: Detent;
  heights: Heights;
  onDetentChange: (detent: Detent) => void;
}): Handle {
  const advance = useCallback(() => {
    onDetentChange(nextOnPress(detent));
  }, [detent, onDetentChange]);
  const press = useCallback(
    (event: MouseEvent) => {
      // A pointer's click has a count; the keyboard's is 0.
      if (event.detail === 0) {
        advance();
      }
    },
    [advance],
  );
  const keys = useCallback(
    (event: KeyboardEvent) => {
      const direction = KEY_STEPS[event.key];
      if (direction === undefined) {
        return;
      }
      event.preventDefault();
      const all = DETENTS.map((each) => heights[each]);
      onDetentChange(detentAt(stepDetent(heights[detent], all, direction), heights));
    },
    [detent, heights, onDetentChange],
  );
  return { advance, press, keys };
}

/**
 * The phone shell's sheet (DESIGN.md §10 shells, after Main.dc.html): three detents, dragged by
 * its header with momentum, on `floating` material. The map behind stays live at peek and medium.
 * The sheet is laid out at the large detent and moved down by a transform; its scroll area is sized
 * to the height it rests at, so the end of a list is always reachable.
 */
function BottomSheet({ label, Header, children, detent, onDetentChange, onRest }: BottomSheetProps): ReactElement {
  const sheet = useRef<HTMLElement>(null);
  const top = useRef<HTMLDivElement>(null);
  const container = useHeight(sheet, true);
  const headerHeight = useHeight(top);
  const heights = useMemo(() => detentHeights(container, headerHeight), [container, headerHeight]);
  const motion = useSheetMotion(sheet, heights.large);
  const measured = container > 0 && headerHeight > 0;
  const { rested, settle } = useSheetPlacement({ motion, heights, measured, detent, onDetentChange, onRest });
  const { advance, press, keys } = useHandle({ detent, heights, onDetentChange });
  const { onPointerDown, onPointerMove, onPointerUp, onPointerCancel } = useSheetDrag(motion, heights, {
    settle,
    tap: advance,
  });
  const sheetStyle = useMemo<CSSProperties>(
    () => ({ height: heights.large, visibility: measured ? "visible" : "hidden" }),
    [heights.large, measured],
  );
  const bodyStyle = useMemo<CSSProperties>(
    () => ({ height: Math.max(0, rested - headerHeight) }),
    [rested, headerHeight],
  );

  return (
    <section
      ref={sheet}
      aria-label={label}
      style={sheetStyle}
      className="absolute inset-x-0 bottom-0 z-20 flex flex-col rounded-t-lg border-t border-hairline-strong shadow-floating material-floating will-change-transform contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <div
        ref={top}
        className="shrink-0 touch-none select-none"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
      >
        <button
          type="button"
          data-sheet-handle
          aria-label={`Resize sheet, now ${DETENT_WORDS[detent]}`}
          onClick={press}
          onKeyDown={keys}
          className="flex h-6 w-full cursor-grab items-center justify-center rounded-t-lg focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent active:cursor-grabbing"
        >
          <span className="h-1 w-9 rounded-pill bg-ink/30" aria-hidden />
        </button>
        <Header />
      </div>
      <div className="min-h-0 overflow-y-auto overscroll-contain" style={bodyStyle}>
        {children}
      </div>
    </section>
  );
}

export { BottomSheet };

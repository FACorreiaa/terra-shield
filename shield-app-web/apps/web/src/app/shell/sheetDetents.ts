/** The sheet's three detents (DESIGN.md §10 shells). */
import { createContext } from "react";

type Detent = "peek" | "medium" | "large";

const DETENTS: readonly Detent[] = ["peek", "medium", "large"];
/** Medium is half the screen and large nearly all of it, leaving the capsule's strip of map. */
const SHARE = { medium: 0.5, large: 0.92 } as const;

type Heights = Readonly<Record<Detent, number>>;

/**
 * Each detent's visible height in a container this tall. Peek is measured (the header with search
 * and the compact strip), so it follows the text size; it never exceeds medium.
 */
function detentHeights(container: number, peek: number): Heights {
  const large = Math.round(container * SHARE.large);
  const medium = Math.max(Math.min(peek, large), Math.round(container * SHARE.medium));
  return { peek: Math.min(peek, medium), medium, large };
}

/** The detent whose height is `height`, or the closest one. */
function detentAt(height: number, heights: Heights): Detent {
  let best: Detent = "peek";
  for (const detent of DETENTS) {
    if (Math.abs(heights[detent] - height) < Math.abs(heights[best] - height)) {
      best = detent;
    }
  }
  return best;
}

/** The order a press on the handle walks: up to large, then back down to peek. */
const ON_PRESS: Readonly<Record<Detent, Detent>> = { peek: "medium", medium: "large", large: "peek" };

function nextOnPress(detent: Detent): Detent {
  return ON_PRESS[detent];
}

/** The phone sheet's resting detent, for the header, which the sheet draws without props. */
const RestingDetent = createContext<Detent>("peek");

/** Moves the phone sheet from inside it ("Show on map" lowers it); undefined on a wide screen. */
const SheetDetentSetter = createContext<((detent: Detent) => void) | undefined>(undefined);

export type { Detent, Heights };
export { detentAt, detentHeights, DETENTS, nextOnPress, RestingDetent, SheetDetentSetter };

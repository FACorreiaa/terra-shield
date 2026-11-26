import type { PaddingOptions } from "maplibre-gl";

/**
 * How much of the map the shell's chrome covers on each side (the phone's sheet and capsule, the
 * desktop inspector), so camera moves centre in the part that is actually visible. The shell writes
 * it; fits read it when they run. Not reactive: changing it never moves the camera by itself.
 */
const insets: Required<PaddingOptions> = { top: 0, right: 0, bottom: 0, left: 0 };

function mapInsets(): Required<PaddingOptions> {
  return { ...insets };
}

function setMapInsets(next: Partial<PaddingOptions>): void {
  Object.assign(insets, next);
}

/** The insets with `extra` px of breathing room on every side. */
function insetsPlus(extra: number): PaddingOptions {
  return {
    top: insets.top + extra,
    right: insets.right + extra,
    bottom: insets.bottom + extra,
    left: insets.left + extra,
  };
}

/** A camera move waiting for the chrome to settle, and whether the chrome is moving now. */
const chrome: { moving: boolean; waiting: (() => void) | undefined } = { moving: false, waiting: undefined };

function flush(): void {
  if (chrome.moving) {
    return;
  }
  const move = chrome.waiting;
  chrome.waiting = undefined;
  move?.();
}

/**
 * The shell says when its chrome starts moving (the phone's sheet springing to a detent) and when
 * it has come to rest with its insets written; at rest, a waiting camera move runs.
 */
function setChromeMoving(moving: boolean): void {
  chrome.moving = moving;
  flush();
}

/**
 * Runs `move` once the chrome has settled: after this frame's layout (an inspector closing), and
 * after a sheet that started moving has come to rest. Fitting earlier would use the insets of the
 * chrome as it was; the apps wait for the sheet the same way. A newer move replaces a waiting one.
 */
function whenChromeRests(move: () => void): void {
  chrome.waiting = move;
  requestAnimationFrame(flush);
}

export { insetsPlus, mapInsets, setChromeMoving, setMapInsets, whenChromeRests };

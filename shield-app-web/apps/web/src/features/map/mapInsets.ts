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

export { insetsPlus, mapInsets, setMapInsets };

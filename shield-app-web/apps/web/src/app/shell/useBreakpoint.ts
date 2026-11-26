import { useSyncExternalStore } from "react";

/** DESIGN.md §10 shells: at 1024 px and up the web mirrors macOS, below it mirrors iOS. */
const WIDE = "(min-width: 1024px)";

function subscribe(onChange: () => void): () => void {
  const query = globalThis.matchMedia(WIDE);
  query.addEventListener("change", onChange);
  return () => {
    query.removeEventListener("change", onChange);
  };
}

function isWide(): boolean {
  return globalThis.matchMedia(WIDE).matches;
}

/** True for the desktop shell (sidebar · map · inspector), false for the phone shell (map · sheet). */
function useIsWide(): boolean {
  return useSyncExternalStore(subscribe, isWide, () => true);
}

export { useIsWide };

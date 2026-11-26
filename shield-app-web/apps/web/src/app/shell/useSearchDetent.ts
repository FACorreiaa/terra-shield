import { useState } from "react";

import { useMapSearch } from "./mapSearchContext.ts";
import type { Detent } from "./sheetDetents.ts";

/**
 * Searching opens the sheet to full height, where the results have room (DESIGN.md §10 shells).
 * Ending it puts the sheet back where it was, if it is still at full height (a sheet dragged
 * elsewhere meanwhile stays put, as in MapScreen.swift); after choosing an event, at least at half
 * height so the event shows. Adjusted during render, after the selection's own lift, so this has
 * the last word.
 */
function useSearchDetent(detent: Detent, setDetent: (detent: Detent) => void, selectedId: string | undefined): void {
  const { active } = useMapSearch();
  const [searching, setSearching] = useState(active);
  const [before, setBefore] = useState<{ detent: Detent; selectedId: string | undefined }>({ detent, selectedId });
  if (active !== searching) {
    setSearching(active);
    if (active) {
      setBefore({ detent, selectedId });
      setDetent("large");
    } else if (detent === "large") {
      const chose = selectedId !== undefined && selectedId !== before.selectedId;
      setDetent(chose && before.detent === "peek" ? "medium" : before.detent);
    }
  }
}

export { useSearchDetent };

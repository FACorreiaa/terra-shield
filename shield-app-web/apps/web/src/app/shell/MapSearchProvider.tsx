import type { ReactElement, ReactNode } from "react";
import { useCallback, useMemo, useRef, useState } from "react";

import type { MapSearch } from "./mapSearchContext.ts";
import { MapSearchContext, SEARCH_AREA } from "./mapSearchContext.ts";
import { useIsWide } from "./useBreakpoint.ts";
import { usePage } from "./usePage.ts";

/** Whether keyboard focus is in the field or on one of its results. */
function focusInSearch(): boolean {
  return Boolean(document.activeElement?.closest(`[${SEARCH_AREA}]`));
}

/**
 * Holds the search's text and focus for whichever shell draws the field. On a phone, focusing the
 * field opens the results, as on iPhone; on a wide screen they wait for text, as on the Mac, so
 * the list stays and Tab walks on into it (MapSearch.swift).
 */
function MapSearchProvider({ children }: { children: ReactNode }): ReactElement {
  const wide = useIsWide();
  const [text, setText] = useState("");
  const [editing, setEditing] = useState(false);
  const field = useRef<HTMLInputElement>(null);
  const end = useCallback(() => {
    setText("");
    setEditing(false);
    // A phone drops the keyboard. A wide screen keeps keyboard focus in the sidebar, on the field,
    // rather than losing it to the page when the results it sat on go away.
    if (!wide) {
      field.current?.blur();
    } else if (focusInSearch()) {
      field.current?.focus();
    }
  }, [wide]);
  const focus = useCallback(() => {
    field.current?.focus();
    return field.current !== null;
  }, []);
  // The Countries list hides the field, so a search carried there from another section ends.
  // Adjusted during render; the field is gone, so there is no focus to look after.
  const { listing } = usePage();
  const [wasListing, setWasListing] = useState(listing);
  if (listing !== wasListing) {
    setWasListing(listing);
    if (listing) {
      setText("");
      setEditing(false);
    }
  }
  const value = useMemo<MapSearch>(
    () => ({ text, setText, editing, setEditing, active: (editing && !wide) || text.trim() !== "", end, focus, field }),
    [text, editing, wide, end, focus],
  );
  return <MapSearchContext value={value}>{children}</MapSearchContext>;
}

export { MapSearchProvider };

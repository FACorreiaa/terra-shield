import type { RefObject } from "react";
import { createContext, use } from "react";

/** The live map's search (MapSearch.swift): one field, in the sidebar on a wide screen, the sheet's header on a phone. */
interface MapSearch {
  text: string;
  setText: (text: string) => void;
  /** The field has focus. */
  editing: boolean;
  setEditing: (editing: boolean) => void;
  /** Results show while there is text, and on a phone while the field has focus. */
  active: boolean;
  /** Clears the text and ends editing: a phone lets go of the field, a wide screen keeps focus on it. */
  end: () => void;
  /** Puts the cursor in the field; false when no field is on screen. */
  focus: () => boolean;
  field: RefObject<HTMLInputElement | null>;
}

/** Marks the field and its results, so ending a search knows whether keyboard focus was there. */
const SEARCH_AREA = "data-search-area";

const MapSearchContext = createContext<MapSearch | undefined>(undefined);

function useMapSearch(): MapSearch {
  const value = use(MapSearchContext);
  if (value === undefined) {
    throw new Error("useMapSearch needs a MapSearchProvider above it");
  }
  return value;
}

export type { MapSearch };
export { MapSearchContext, SEARCH_AREA, useMapSearch };

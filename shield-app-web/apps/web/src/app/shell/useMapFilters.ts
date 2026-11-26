import type { EventQuery } from "@/domain/filters.ts";
import type { View } from "@/domain/viewport.ts";
import type { Detections } from "@/features/detections/useDetections.ts";
import { useDetections } from "@/features/detections/useDetections.ts";
import { setQuery, useSavedFilters } from "@/features/filters/filterPrefs.ts";

interface MapFilters {
  query: EventQuery;
  setQuery: (query: EventQuery) => void;
  /** Satellite heat detections for the view, while the saved switch has them on. */
  detections: Detections;
}

/** What the person chose to see (`shield.filters`): the events' filters and the heat layer. */
function useMapFilters(view: View): MapFilters {
  const { query, showsDetections } = useSavedFilters();
  const detections = useDetections(view, showsDetections);
  return { query, setQuery, detections };
}

export { useMapFilters };

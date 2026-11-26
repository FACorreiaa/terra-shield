import type { EventQuery } from "@/domain/filters.ts";
import type { View } from "@/domain/viewport.ts";
import type { Conflicts } from "@/features/conflicts/useConflicts.ts";
import { useConflicts } from "@/features/conflicts/useConflicts.ts";
import type { Detections } from "@/features/detections/useDetections.ts";
import { useDetections } from "@/features/detections/useDetections.ts";
import { setQuery, useSavedFilters } from "@/features/filters/filterPrefs.ts";

interface MapFilters {
  query: EventQuery;
  setQuery: (query: EventQuery) => void;
  /** Satellite heat detections for the view, while the saved switch has them on. */
  detections: Detections;
  /** Armed conflicts for the view, while that saved switch has them on (ADR 27). */
  conflicts: Conflicts;
}

/** What the person chose to see (`shield.filters`): the events' filters, the heat and conflicts layers. */
function useMapFilters(view: View): MapFilters {
  const { query, showsDetections, showsConflicts } = useSavedFilters();
  const detections = useDetections(view, showsDetections);
  const conflicts = useConflicts(view, showsConflicts);
  return { query, setQuery, detections, conflicts };
}

export type { MapFilters };
export { useMapFilters };

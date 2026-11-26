import type { HazardEvent } from "@shield/api-client";
import { useMemo } from "react";

import { CONFLICT } from "@/domain/taxonomy.ts";

import { useSelectedEvent } from "./useSelectedEvent.ts";

/** Hazards, then the conflicts layer's: what selects, and what ↑ ↓ step through, in list order. */
function knownEvents(list: readonly HazardEvent[], conflicts: readonly HazardEvent[]): readonly HazardEvent[] {
  return conflicts.length === 0 ? list : [...list, ...conflicts];
}

interface Drawing {
  /** The selection's source no longer publishes it. */
  gone: boolean;
  conflictsShown: boolean;
}

/**
 * What the map draws: every known event, and the selection when neither list has it (a shared
 * link, a country's event), but not once its source has dropped it, and not a conflict while the
 * conflicts layer is off (`apply(conflicts: nil)`, HazardMapView.swift): its detail stays open.
 */
function drawnEvents(
  known: readonly HazardEvent[],
  selected: HazardEvent | undefined,
  { gone, conflictsShown }: Drawing,
): readonly HazardEvent[] {
  if (selected === undefined || gone || known.some((event) => event.id === selected.id)) {
    return known;
  }
  if (selected.type === CONFLICT && !conflictsShown) {
    return known;
  }
  return [...known, selected];
}

interface ConflictLayer {
  shown: boolean;
  events: readonly HazardEvent[];
}

interface Selection {
  selected: HazardEvent | undefined;
  detail: ReturnType<typeof useSelectedEvent>["detail"];
  selectedShape: ReturnType<typeof useSelectedEvent>["selectedShape"];
  known: readonly HazardEvent[];
  drawn: readonly HazardEvent[];
}

/**
 * The selection, looked up among hazards and conflicts alike (LiveMapModel `selectedEvent`), and
 * what the map draws with it.
 */
function useLayerSelection(
  { selectedId, chosen }: { selectedId: string | undefined; chosen: HazardEvent | undefined },
  list: readonly HazardEvent[],
  conflicts: ConflictLayer,
): Selection {
  const known = useMemo(() => knownEvents(list, conflicts.events), [list, conflicts.events]);
  const { selected, detail, selectedShape } = useSelectedEvent(selectedId, known, chosen);
  const gone = detail?.kind === "gone";
  const drawn = useMemo(
    () => drawnEvents(known, selected, { gone, conflictsShown: conflicts.shown }),
    [known, selected, gone, conflicts.shown],
  );
  return { selected, detail, selectedShape, known, drawn };
}

export { drawnEvents, knownEvents, useLayerSelection };

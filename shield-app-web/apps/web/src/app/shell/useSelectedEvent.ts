import type { HazardEvent } from "@shield/api-client";
import { useMemo, useState } from "react";

import type { EventShape } from "@/domain/geo.ts";
import { eventShape } from "@/domain/geo.ts";
import type { DetailAnswer, DetailState } from "@/features/detail/detailState.ts";
import { detailState } from "@/features/detail/detailState.ts";
import { useEventDetail } from "@/features/detail/useEventDetail.ts";

interface SelectedEvent {
  selected: HazardEvent | undefined;
  detail: DetailState | undefined;
  selectedShape: EventShape | undefined;
}

/**
 * The last answer for this id: a newer copy refetches under a new key, and if that fetch fails the
 * detail already shown stays (LiveMapModel keeps it; a query's placeholder ends with the fetch).
 */
function useHeldAnswer(id: string | undefined, data: DetailAnswer | undefined): DetailAnswer | undefined {
  const [held, setHeld] = useState<{ id: string; answer: DetailAnswer }>();
  if (id !== undefined && data !== undefined && (held?.id !== id || held?.answer !== data)) {
    setHeld({ id, answer: data });
  }
  return data ?? (held !== undefined && held.id === id ? held.answer : undefined);
}

/**
 * The selection in full (LiveMapModel `selectedEvent`): the map's copy, the fresher one, else the
 * detail's own (a shared link, an event panned away or folded into a cluster), else the event as
 * last seen once its source has dropped it. Keyed on the id, so none of those closes the detail.
 */
function useSelectedEvent(selectedId: string | undefined, list: readonly HazardEvent[]): SelectedEvent {
  const listed = useMemo(() => list.find((event) => event.id === selectedId), [list, selectedId]);
  const query = useEventDetail(selectedId, listed?.updatedAt);
  const answer = useHeldAnswer(selectedId, query.data);
  const loaded = answer?.found === true ? answer.detail : undefined;
  const current = listed ?? loaded;
  const [seen, setSeen] = useState<HazardEvent>();
  // Adjusted during render: the last copy seen, for "gone".
  if (current !== undefined && current !== seen) {
    setSeen(current);
  }
  const last = seen?.id === selectedId ? seen : undefined;
  const selectedShape = useMemo(() => (loaded === undefined ? undefined : eventShape(loaded.geometry)), [loaded]);
  return {
    selected: current ?? last,
    detail: selectedId === undefined ? undefined : detailState(answer, query.isError, current ?? last),
    selectedShape,
  };
}

export { useHeldAnswer, useSelectedEvent };

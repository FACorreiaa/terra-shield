import type { EventCluster, EventList, HazardEvent } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";

import type { EventQuery } from "@/domain/filters.ts";
import type { LiveStatus } from "@/domain/liveStatus.ts";
import type { View } from "@/domain/viewport.ts";
import { useEvents } from "@/features/events/useEvents.ts";

import { useLiveChanges } from "./useLiveChanges.ts";
import { useDisplayed, useShownEvents } from "./useShownEvents.ts";

interface MapEventsInput {
  view: View;
  query: EventQuery;
  selectedId: string | undefined;
  now: Date;
  onArrival: (event: HazardEvent, animate: boolean) => void;
}

interface MapEvents {
  events: UseQueryResult<EventList>;
  shown: EventList | undefined;
  list: readonly HazardEvent[];
  clusters: readonly EventCluster[];
  status: LiveStatus;
  failure: string | undefined;
}

/** The map's events: loaded for the view and filters, kept current by the live stream, shown through failures. */
function useMapEvents(input: MapEventsInput): MapEvents {
  const { view, query, now } = input;
  const events = useEvents(view, query);
  const liveAt = useLiveChanges({ ...input, events });
  const { shown, status, failure } = useShownEvents(events, now, liveAt);
  const { list, clusters } = useDisplayed(shown);
  return { events, shown, list, clusters, status, failure };
}

export { useMapEvents };

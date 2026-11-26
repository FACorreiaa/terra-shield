import type { EventCluster, EventList, HazardEvent } from "@shield/api-client";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { CONFLICTS_QUERY, toApiQuery } from "@/domain/filters.ts";
import { sortedForDisplay } from "@/domain/sort.ts";
import type { View } from "@/domain/viewport.ts";
import { requestEvents } from "@/features/events/useEvents.ts";
import { toggleShowsConflicts } from "@/features/filters/filterPrefs.ts";
import { sameSession, useLastGood, useSession } from "@/features/map/layerSession.ts";

/** The armed-conflicts layer (ADR 27) as the map, its legend and the list need it. */
interface Conflicts {
  /** Whether the person has the layer on. */
  shown: boolean;
  /** The answer shown: undefined while off and before the first answer; through a failure, the last. */
  list: EventList | undefined;
  /** Its events in display order (most severe, then newest); none while off. */
  events: readonly HazardEvent[];
  clusters: readonly EventCluster[];
  /** The last load failed (the legend says so; the events' banner doesn't). */
  failed: boolean;
  /** Loads again now (the refresh shortcut); nothing while the layer is off. */
  refetch: () => void;
  /** Turns the layer on or off (the `c` shortcut). */
  toggle: () => void;
}

/**
 * 6 h: UCDP is a monthly release, checked every 6 h. A pan to a new view still asks at once; `r` asks
 * again now.
 */
const CONFLICTS_MS = 21_600_000;

const NO_EVENTS: readonly HazardEvent[] = [];
const NO_CLUSTERS: readonly EventCluster[] = [];

/**
 * `GET /v1/events?type=conflict` for the map's view while the layer is on (LiveMapModel
 * `conflicts`): their own answer, server-clustered below zoom 6, never in the hazards' counts, list
 * or clusters. No window: the server keeps a conflict 90 days after its end date. Not streamed, and
 * its key isn't `events`, so the live stream never patches or reloads it.
 */
function useConflicts(view: View, shown: boolean): Conflicts {
  const session = useSession(shown);
  const query = useQuery({
    queryKey: ["conflicts", session, view.zoom, view.bbox],
    queryFn: ({ signal }) =>
      requestEvents(
        {
          ...toApiQuery(CONFLICTS_QUERY),
          zoom: view.zoom,
          ...(view.bbox === undefined ? {} : { bbox: view.bbox }),
        },
        signal,
      ),
    enabled: shown,
    placeholderData: (previous, previousQuery) => (sameSession(previousQuery, session) ? previous : undefined),
    staleTime: CONFLICTS_MS,
    refetchInterval: CONFLICTS_MS,
  });
  const last = useLastGood(query, shown, session);
  const list = shown ? (query.data ?? last) : undefined;
  const failed = shown && query.isError;
  const { refetch } = query;
  const events = useMemo(() => (list === undefined ? NO_EVENTS : sortedForDisplay(list.events)), [list]);
  return useMemo(
    () => ({
      shown,
      list,
      events,
      clusters: list?.clusters ?? NO_CLUSTERS,
      failed,
      refetch: () => {
        if (shown) {
          void refetch();
        }
      },
      toggle: toggleShowsConflicts,
    }),
    [shown, list, events, failed, refetch],
  );
}

export type { Conflicts };
export { useConflicts };

import type { AircraftList } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { aircraftAllowed } from "@/domain/viewport.ts";
import type { View } from "@/domain/viewport.ts";
import { sameSession, useSession } from "@/features/map/layerSession.ts";
import { api } from "@/lib/api/client.ts";

/** The live flights layer as the map, its legend and the detail need it. */
interface Flights {
  /** Whether the layer is on: off at every load, never saved (ADR 28). */
  shown: boolean;
  /**
   * What to draw: this view's answer, or the last view's while this one loads; nothing while off,
   * zoomed out, or once a new view's load failed (another view's positions would count as this one's).
   */
  list: AircraftList | undefined;
  /** The last load failed: for the same view, the positions shown are the previous answer's. */
  failed: boolean;
  /** The view is larger than the server serves (four 5° cells, or its 400): "zoom in". */
  needsZoom: boolean;
}

/**
 * One request a minute at most (the server's cells refresh no faster, ADR 28), and none while the
 * tab is hidden: TanStack's interval pauses in the background, as the apps' task does.
 */
const FLIGHTS_MS = 60_000;

class FlightsError extends Error {
  public override readonly name = "FlightsError";

  /** The server refused the view as larger than four cells (a 400): "zoom in". */
  public readonly tooLarge: boolean;

  public constructor(message: string, tooLarge = false) {
    super(message);
    this.tooLarge = tooLarge;
  }
}

const BAD_REQUEST = 400;

async function requestAircraft(bbox: string, signal: AbortSignal): Promise<AircraftList> {
  const { data, error, response } = await api.GET("/v1/aircraft", { params: { query: { bbox } }, signal });
  if (response.status === BAD_REQUEST) {
    throw new FlightsError("Zoom in to see aircraft", true);
  }
  if (error !== undefined) {
    throw new FlightsError("Flights didn’t load");
  }
  return data;
}

type Answer = UseQueryResult<AircraftList>;

interface AircraftAsk {
  bbox: string | undefined;
  enabled: boolean;
  /** The layer's session: turned off and on, it starts with nothing. */
  session: number;
  /** The view last answered (or refused): the only one whose positions a new view may show while it loads. */
  settled: string | undefined;
}

/**
 * One request a minute and no more (Flights.swift): a failure waits for the next minute, as does a
 * return to the tab or the network; a new view asks at once.
 */
function useAircraft({ bbox, enabled, session, settled }: AircraftAsk): Answer {
  return useQuery({
    queryKey: ["aircraft", session, bbox],
    queryFn: ({ signal }) => requestAircraft(bbox ?? "", signal),
    enabled,
    placeholderData: (previous, previousQuery) =>
      sameSession(previousQuery, session) && previousQuery?.queryKey[2] === settled ? previous : undefined,
    staleTime: FLIGHTS_MS,
    refetchInterval: FLIGHTS_MS,
    // Positions go stale in minutes: an old view's are of no use when panning back.
    gcTime: FLIGHTS_MS,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

/**
 * The query for one view. A new view shows the last answered view's positions while it loads, and
 * only that view's: TanStack offers the last view that had data, which after a failure, a refusal
 * or a zoom-out is an older one, whose aircraft would count as this view's.
 */
function useAircraftQuery(bbox: string | undefined, enabled: boolean, session: number): Answer {
  const [settled, setSettled] = useState<string>();
  const query = useAircraft({ bbox, enabled, session, settled });
  // The view last answered, or refused, in this session (adjusted during render, not in an effect).
  const answered = query.isFetched && !query.isPlaceholderData;
  let next = enabled ? settled : undefined;
  if (enabled && answered) {
    next = bbox;
  }
  if (next !== settled) {
    setSettled(next);
  }
  return query;
}

/**
 * `GET /v1/aircraft` for the map's view while the layer is on (LiveMapModel `startFlights`): never
 * for a view the server would refuse, at once for a new view, then every minute.
 */
function useFlights(view: View, shown: boolean): Flights {
  const session = useSession(shown);
  const { bbox } = view;
  const allowed = bbox !== undefined && aircraftAllowed(bbox);
  const query = useAircraftQuery(bbox, shown && allowed, session);
  const needsZoom = shown && (!allowed || (query.error instanceof FlightsError && query.error.tooLarge));
  const live = shown && !needsZoom;
  const list = live ? query.data : undefined;
  const failed = live && query.isError;
  return useMemo(() => ({ shown, list, failed, needsZoom }), [shown, list, failed, needsZoom]);
}

export type { Flights };
export { FLIGHTS_MS, useFlights };

import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import type { DetectionLayer } from "@/domain/detections.ts";
import { detectionLayer, EMPTY_DETECTIONS, sameDetections } from "@/domain/detections.ts";
import type { View } from "@/domain/viewport.ts";
import { toggleShowsDetections } from "@/features/filters/filterPrefs.ts";
import { sameSession, useLastGood, useSession } from "@/features/map/layerSession.ts";
import { api } from "@/lib/api/client.ts";

/** The heat layer as the map and its legend need it. */
interface Detections {
  /** Whether the person has the layer on. */
  shown: boolean;
  /** What to draw: empty when off; through a failed load, the last layer that arrived. */
  layer: DetectionLayer;
  /** The last load failed (the legend says so; the events' banner doesn't). */
  failed: boolean;
  loading: boolean;
  /** Loads again now (the refresh shortcut); nothing while the layer is off. */
  refetch: () => void;
  /** Turns the layer on or off (the `h` shortcut). */
  toggle: () => void;
}

/** The server caches `/v1/fires` for 60 s, and FIRMS itself updates every 15 min at most. */
const FIRES_MS = 60_000;

/** A parsed answer, as `structuralSharing` sees the cache's data (typed `unknown` there). */
function isLayer(data: unknown): data is DetectionLayer {
  return typeof data === "object" && data !== null && "latest" in data && "kind" in data;
}

class DetectionsError extends Error {
  public override readonly name = "DetectionsError";
}

async function requestFires(view: View, signal: AbortSignal): Promise<DetectionLayer> {
  // No `since`: the server's 24 h default, as the apps. The filters' window is for events.
  const { data, error } = await api.GET("/v1/fires", {
    params: { query: { zoom: view.zoom, ...(view.bbox === undefined ? {} : { bbox: view.bbox }) } },
    signal,
  });
  if (error !== undefined) {
    throw new DetectionsError("Heat detections didn’t load");
  }
  return detectionLayer(data);
}

/** Inactive answers go soon: at zoom 6 and up one can hold tens of thousands of points. */
const FIRES_GC_MS = 30_000;

/**
 * `GET /v1/fires` for the map's view while the heat layer is on (LiveMapModel `detections`): grid
 * cells below zoom 6, single detections from 6. A failed load keeps the last layer on the map.
 */
function useDetections(view: View, shown: boolean): Detections {
  const session = useSession(shown);
  const fires = useQuery({
    queryKey: ["fires", session, view.zoom, view.bbox],
    queryFn: ({ signal }) => requestFires(view, signal),
    enabled: shown,
    // Across a pan, the last view's layer, but never one from before the layer was last turned on.
    placeholderData: (previous, previousQuery) => (sameSession(previousQuery, session) ? previous : undefined),
    staleTime: FIRES_MS,
    refetchInterval: FIRES_MS,
    gcTime: FIRES_GC_MS,
    // A reload that draws the same keeps the same layer, so the map isn't handed it again.
    structuralSharing: (previous, next) =>
      isLayer(previous) && isLayer(next) && sameDetections(previous, next) ? previous : next,
  });
  const last = useLastGood(fires, shown, session);
  const layer = shown ? (fires.data ?? last ?? EMPTY_DETECTIONS) : EMPTY_DETECTIONS;
  const failed = shown && fires.isError;
  const loading = shown && fires.isFetching;
  const { refetch } = fires;
  return useMemo(
    () => ({
      shown,
      layer,
      failed,
      loading,
      refetch: () => {
        if (shown) {
          void refetch();
        }
      },
      toggle: toggleShowsDetections,
    }),
    [shown, layer, failed, loading, refetch],
  );
}

export type { Detections };
export { useDetections };

import type { EventCluster, HazardEvent } from "@shield/api-client";
import type { ReactElement, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import type { View } from "@/domain/viewport.ts";
import { useEvents } from "@/features/events/useEvents.ts";

import type { LiveMap } from "./liveMapContext.ts";
import { LiveMapContext } from "./liveMapContext.ts";

const NO_EVENTS: readonly HazardEvent[] = [];
const NO_CLUSTERS: readonly EventCluster[] = [];
const WHOLE_WORLD: View = { zoom: 1, bbox: "-180,-85,180,85" };
const CLOCK_TICK_MS = 30_000;

function useNow(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(new Date());
    }, CLOCK_TICK_MS);
    return () => {
      clearInterval(timer);
    };
  }, []);
  return now;
}

/**
 * Holds the live map's state above the routes and both shells, so neither a route change nor
 * crossing the 1024 px breakpoint resets the view or the selection. W5 moves the selection into
 * the URL (`/e/$id`) behind the same `select`.
 */
function LiveMapProvider({ children }: { children: ReactNode }): ReactElement {
  const [view, setView] = useState<View>(WHOLE_WORLD);
  const [selectedId, setSelectedId] = useState<string>();
  const now = useNow();
  const events = useEvents(view);
  const list = events.data?.events ?? NO_EVENTS;
  const clusters = events.data?.clusters ?? NO_CLUSTERS;
  const selected = useMemo(() => list.find((event) => event.id === selectedId), [list, selectedId]);
  const value = useMemo<LiveMap>(
    () => ({ view, setView, selectedId, select: setSelectedId, selected, events, list, clusters, now }),
    [view, selectedId, selected, events, list, clusters, now],
  );
  return <LiveMapContext value={value}>{children}</LiveMapContext>;
}

export { LiveMapProvider };

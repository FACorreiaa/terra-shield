import type { EventCluster, HazardEvent } from "@shield/api-client";
import { createFileRoute } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { useCallback, useEffect, useState } from "react";

import { Button } from "@/components/ui/button.tsx";
import type { View } from "@/domain/viewport.ts";
import { EventList } from "@/features/events/EventList.tsx";
import { useEvents } from "@/features/events/useEvents.ts";
import { HazardMap } from "@/features/map/HazardMap.tsx";

const NO_EVENTS: readonly HazardEvent[] = [];
const NO_CLUSTERS: readonly EventCluster[] = [];
/** Source health arrives with the sources view (W4); until then nothing is marked stale. */
const NO_STALE_SOURCES: ReadonlySet<string> = new Set();
const WHOLE_WORLD: View = { zoom: 1, bbox: "-180,-85,180,85" };
/** Relative times ("12 min ago") re-render this often. */
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

function ApiUnreachable({ reason, onRetry }: { reason: string; onRetry: () => void }): ReactElement {
  return (
    <div className="flex flex-col items-start gap-3 px-4 py-6">
      <p className="text-body text-ink">Can’t reach Shield’s API.</p>
      <p className="text-meta text-muted">{reason}</p>
      <Button size="sm" onClick={onRetry}>
        Try again
      </Button>
    </div>
  );
}

/** The live map (DESIGN.md §10.1): map canvas plus the event list in a side panel. */
function MonitorPage(): ReactElement {
  const [view, setView] = useState<View>(WHOLE_WORLD);
  const [selectedId, setSelectedId] = useState<string>();
  const now = useNow();
  const events = useEvents(view);
  const { refetch } = events;
  const retry = useCallback(() => {
    void refetch();
  }, [refetch]);

  const list = events.data?.events ?? NO_EVENTS;
  const clusters = events.data?.clusters ?? NO_CLUSTERS;
  const count = events.data === undefined ? "–" : `${list.length}${events.data.truncated ? "+" : ""}`;
  return (
    <div className="absolute inset-0 flex">
      <aside className="z-10 flex w-96 shrink-0 flex-col border-r border-ink/8 bg-panel max-md:hidden">
        <div className="flex items-baseline justify-between px-4 py-3">
          <h1 className="text-title text-ink">Live events</h1>
          <span className="font-mono text-meta text-muted">{count}</span>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {events.isError ? (
            <ApiUnreachable reason={events.error.message} onRetry={retry} />
          ) : (
            <EventList events={list} selectedId={selectedId} onSelect={setSelectedId} now={now} />
          )}
          {clusters.length > 0 ? (
            <p className="px-4 py-3 text-meta text-muted">Zoom in to list events inside the clusters.</p>
          ) : undefined}
        </div>
      </aside>
      <div className="relative min-w-0 flex-1">
        <HazardMap
          events={list}
          clusters={clusters}
          staleSources={NO_STALE_SOURCES}
          selectedId={selectedId}
          selectedShape={undefined}
          projection="globe"
          onViewChange={setView}
          onSelect={setSelectedId}
        />
      </div>
    </div>
  );
}

export const Route = createFileRoute("/_app/map")({ component: MonitorPage });

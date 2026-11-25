import { createFileRoute } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { useLiveMap } from "@/app/shell/liveMapContext.ts";
import { useIsWide } from "@/app/shell/useBreakpoint.ts";
import { Button } from "@/components/ui/button.tsx";
import { EventList } from "@/features/events/EventList.tsx";

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

/**
 * Monitor (DESIGN.md §10.1): the events in view, most severe first. It is the sidebar's body on a
 * wide screen and the sheet's on a phone, where the sheet's header already names it.
 */
function MonitorSection(): ReactElement {
  const { events, list, clusters, selectedId, select, now } = useLiveMap();
  const wide = useIsWide();
  const { refetch } = events;
  const retry = useCallback(() => {
    void refetch();
  }, [refetch]);
  const count = events.data === undefined ? "–" : `${list.length}${events.data.truncated ? "+" : ""}`;

  return (
    <section aria-label="Live events">
      {wide ? (
        <div className="flex items-baseline justify-between px-4 py-3">
          <h1 className="text-title text-ink">Live events</h1>
          <span className="font-mono text-meta text-muted">{count}</span>
        </div>
      ) : undefined}
      {events.isError ? (
        <ApiUnreachable reason={events.error.message} onRetry={retry} />
      ) : (
        <EventList events={list} selectedId={selectedId} onSelect={select} now={now} />
      )}
      {clusters.length > 0 ? (
        <p className="px-4 py-3 text-meta text-muted">Zoom in to list events inside the clusters.</p>
      ) : undefined}
    </section>
  );
}

export const Route = createFileRoute("/_app/map")({ component: MonitorSection });

import type { ReactElement } from "react";
import { useCallback } from "react";

import { conflictCount } from "@/domain/conflicts.ts";
import { DEFAULT_QUERY, isDefault } from "@/domain/filters.ts";
import { listFootnote } from "@/domain/listNotes.ts";
import { ConflictSection } from "@/features/conflicts/ConflictSection.tsx";
import { EventList } from "@/features/events/EventList.tsx";
import { EmptyView, LoadingRows, NothingLoaded } from "@/features/monitor/ListStates.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useRetry } from "./useRetry.ts";
import { useZoomOut } from "./useZoomOut.ts";

/** The list, or why there is none: loading, nothing loaded, or an empty view. */
function MonitorBody(): ReactElement {
  const { shown, events, list, clusters, selectedId, select, now, failure, status, query, setQuery, conflicts } =
    useLiveMap();
  const retry = useRetry();
  const zoomOut = useZoomOut();
  const reset = useCallback(() => {
    setQuery({ ...DEFAULT_QUERY, country: query.country });
  }, [setQuery, query.country]);
  // Offline or failed: the words for that, never a claim about a view that didn't load.
  const problem = status.retry ? "You’re offline." : failure;
  const empty = list.length === 0 && clusters.length === 0;
  if (shown === undefined || (empty && (problem !== undefined || events.isPlaceholderData))) {
    return problem === undefined ? <LoadingRows /> : <NothingLoaded message={problem} onRetry={retry} />;
  }
  if (empty && (conflicts.list === undefined || conflictCount(conflicts.list) === 0)) {
    return <EmptyView filtered={!isDefault(query)} onReset={reset} onZoomOut={zoomOut} />;
  }
  const clustered = clusters.reduce((sum, cluster) => sum + cluster.count, 0);
  const notes = listFootnote({ clustered, listed: list.length, truncated: shown.truncated });
  return (
    <>
      {empty ? undefined : <EventList events={list} selectedId={selectedId} onSelect={select} now={now} />}
      {notes.map((note) => (
        <p key={note} className="px-4 py-3 text-meta text-muted">
          {note}
        </p>
      ))}
      <ConflictSection conflicts={conflicts} selectedId={selectedId} onSelect={select} now={now} noHazards={empty} />
    </>
  );
}

export { MonitorBody };

import type { ReactElement } from "react";
import { useCallback } from "react";

import { toggling } from "@/domain/filters.ts";
import type { StatFilter } from "@/domain/statStrip.ts";
import { liveMapCells } from "@/domain/statStrip.ts";
import { FilterChips } from "@/features/filters/FilterChips.tsx";
import { StatStrip } from "@/features/monitor/StatStrip.tsx";
import { ProgressLine, StatusLine } from "@/features/monitor/StatusLine.tsx";

import { ArrivalToast } from "./ArrivalToast.tsx";
import { useLiveMap } from "./liveMapContext.ts";
import { MapNotices } from "./MapNotices.tsx";
import { useRetry } from "./useRetry.ts";

/**
 * What floats over the desktop map's top-left (DESIGN.md §10 shells, macOS): the stat strip with
 * the live dot and freshness, the filter chips under it, a severe live arrival's toast, the
 * error banner when a load fails, and the keys of the layers that are on.
 * The wrapper lets presses through to the map; only its parts take them.
 */
function MonitorHud(): ReactElement {
  const { shown, query, setQuery, status, events } = useLiveMap();
  const retry = useRetry();
  const toggle = useCallback(
    (filter: StatFilter) => {
      setQuery(toggling(query, filter));
    },
    [query, setQuery],
  );
  return (
    <div className="pointer-events-none absolute top-3 right-[calc(var(--map-inset-right,0px)+4.5rem)] left-[calc(248px+0.75rem)] z-10 flex flex-col items-start gap-2 *:pointer-events-auto">
      <section
        aria-label="Summary"
        className="relative flex w-[22rem] max-w-full flex-col gap-2 rounded-lg border border-hairline-strong px-4 pt-3 pb-2 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
      >
        <StatStrip cells={liveMapCells(shown, query.includeResolved)} query={query} onToggle={toggle} />
        <StatusLine status={status} onRetry={retry} />
        <ProgressLine active={events.isFetching} />
      </section>
      <div className="max-w-full">
        <FilterChips query={query} onChange={setQuery} />
      </div>
      <ArrivalToast />
      <MapNotices className="max-w-full" />
    </div>
  );
}

export { MonitorHud };

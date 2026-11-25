import type { ReactElement } from "react";

import { ProgressLine, StatusLine } from "@/features/monitor/StatusLine.tsx";

import { Brand } from "./Brand.tsx";
import { useLiveMap } from "./liveMapContext.ts";
import { useRetry } from "./useRetry.ts";

/** The top capsule: nav mark, the live dot and how fresh the map's data is (DESIGN.md §10 shells). */
function StatusCapsule(): ReactElement {
  const { status, events } = useLiveMap();
  const retry = useRetry();
  return (
    <header className="absolute inset-x-3 top-3 z-20 flex h-11 items-center justify-between gap-3 rounded-pill border border-hairline-strong pr-4 pl-2 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none">
      {/* The offline line and Retry need the room; the mark alone still leads home. */}
      <Brand compact={status.retry} />
      <StatusLine status={status} onRetry={retry} />
      <ProgressLine active={events.isFetching} />
    </header>
  );
}

export { StatusCapsule };

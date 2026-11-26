import type { ReactElement } from "react";
import { use, useCallback } from "react";

import { toggling } from "@/domain/filters.ts";
import type { StatFilter } from "@/domain/statStrip.ts";
import { FilterChips } from "@/features/filters/FilterChips.tsx";
import { ErrorBanner } from "@/features/monitor/ErrorBanner.tsx";
import { CompactStatStrip, StatStrip } from "@/features/monitor/StatStrip.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { RestingDetent } from "./sheetDetents.ts";
import { useCells } from "./useCells.ts";
import { useRetry } from "./useRetry.ts";

/** Under the capsule: the filter chips, and the error banner when a load fails (DESIGN.md §10.1). */
function PhoneTopStack(): ReactElement {
  const { query, setQuery, failure } = useLiveMap();
  const retry = useRetry();
  return (
    <div className="pointer-events-none absolute inset-x-3 top-[4.25rem] z-20 flex flex-col gap-2 *:pointer-events-auto">
      <FilterChips query={query} onChange={setQuery} />
      {failure === undefined ? undefined : (
        // Clear of the map controls' column on the right.
        <div className="mr-14">
          <ErrorBanner message={failure} onRetry={retry} />
        </div>
      )}
    </div>
  );
}

/** The full strip at the top of the sheet's body, from half height up (owner, 2026-11-19). */
function SheetStrip(): ReactElement {
  const { query, setQuery } = useLiveMap();
  const cells = useCells();
  const toggle = useCallback(
    (filter: StatFilter) => {
      setQuery(toggling(query, filter));
    },
    [query, setQuery],
  );
  return (
    <div className="border-b border-hairline px-4 pb-3">
      <StatStrip cells={cells} query={query} onToggle={toggle} />
    </div>
  );
}

/**
 * Always in view, even at peek: the title and the compact strip. From half height up the full strip
 * takes over in the sheet's body, so the line hides but keeps its height and the peek never shifts.
 */
function SheetHeader(): ReactElement {
  const detent = use(RestingDetent);
  const cells = useCells();
  return (
    <div className="flex flex-col gap-1 px-4 pb-3">
      <h1 className="text-title text-ink">Live events</h1>
      <div className={detent === "peek" ? undefined : "invisible"}>
        <CompactStatStrip cells={cells} />
      </div>
    </div>
  );
}

export { PhoneTopStack, SheetHeader, SheetStrip };

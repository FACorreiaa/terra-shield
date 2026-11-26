import type { ReactElement } from "react";
import { useCallback } from "react";

import { toggling } from "@/domain/filters.ts";
import type { StatFilter } from "@/domain/statStrip.ts";
import { FilterChips } from "@/features/filters/FilterChips.tsx";
import { StatStrip } from "@/features/monitor/StatStrip.tsx";

import { ArrivalToast } from "./ArrivalToast.tsx";
import { useLiveMap } from "./liveMapContext.ts";
import { MapNotices } from "./MapNotices.tsx";
import { useCells } from "./useCells.ts";

/**
 * Under the capsule: the filter chips, a severe live arrival's toast, the error banner when a load
 * fails (DESIGN.md §10.1), and the keys of the layers that are on.
 */
function PhoneTopStack(): ReactElement {
  const { query, setQuery } = useLiveMap();
  return (
    <div className="pointer-events-none absolute inset-x-3 top-[4.25rem] z-20 flex flex-col gap-2 *:pointer-events-auto">
      <FilterChips query={query} onChange={setQuery} />
      {/* Clear of the map controls' column on the right. */}
      <ArrivalToast className="mr-14" />
      {/* Clear of the map controls' column on the right. */}
      <MapNotices className="mr-14" />
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

export { PhoneTopStack, SheetStrip };

import type { ReactElement } from "react";

import { relative } from "@/domain/freshness.ts";
import { StatStrip } from "@/features/monitor/StatStrip.tsx";
import { useTick } from "@/lib/useTick.ts";

import { useWorldCounts } from "./useWorldCounts.ts";

/** The world's numbers on a panel over the hero art (DESIGN.md §12: never text on art without one). */
function WorldStrip(): ReactElement {
  const { cells, updatedAt, failure } = useWorldCounts();
  const now = useTick();
  let status = "Loading…";
  if (failure !== undefined) {
    status = failure;
  } else if (updatedAt !== undefined) {
    status = `Updated ${relative(updatedAt, now)}`;
  }
  return (
    <section
      aria-labelledby="world-heading"
      className="w-full max-w-md rounded-md border border-hairline-strong px-4 pt-3 pb-2 text-left shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <h2 id="world-heading" className="mb-2 text-micro tracking-[0.08em] text-muted uppercase">
        Worldwide · past 24 h
      </h2>
      <StatStrip cells={cells} />
      <p className="font-mono text-meta text-muted">{status}</p>
    </section>
  );
}

export { WorldStrip };

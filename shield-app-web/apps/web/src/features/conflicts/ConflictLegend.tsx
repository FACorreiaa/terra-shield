import { Crosshair } from "lucide-react";
import type { ReactElement } from "react";

import { CONFLICT_NOTE, conflictHeadline } from "@/domain/conflicts.ts";
import { cn } from "@/lib/utils.ts";

import type { Conflicts } from "./useConflicts.ts";

const PART = " · ";

/**
 * The conflicts layer's key, under the map's notices while the layer is on (`ConflictLegend`,
 * MapScreen.swift): how many are in view, and always that these are monthly research estimates.
 * A failed load says so here, not in the events' banner.
 */
function ConflictLegend({ conflicts }: { conflicts: Conflicts }): ReactElement {
  const { list, failed } = conflicts;
  const headline = conflictHeadline({ list, failed });
  return (
    <div
      data-conflict-legend
      className="flex w-fit max-w-[min(100%,22.5rem)] flex-col gap-1 rounded-md border border-hairline-strong px-3 py-2 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <span className={cn("flex items-start gap-2 text-meta font-semibold", failed ? "text-warning" : "text-ink")}>
        <Crosshair className="mt-0.5 size-3 shrink-0" strokeWidth={2.5} aria-hidden />
        {/* Wraps between parts only, as the heat key does. */}
        <span>
          {headline.split(PART).map((part, index) => (
            <span key={part} className="whitespace-nowrap">
              {index === 0 ? part : `${PART}${part}`}
            </span>
          ))}
        </span>
      </span>
      <span className="text-meta text-muted">{CONFLICT_NOTE}</span>
      {/* A failure is announced once; the count itself stays quiet as the map moves. */}
      <output className="sr-only">{failed ? headline : ""}</output>
    </div>
  );
}

export { ConflictLegend };

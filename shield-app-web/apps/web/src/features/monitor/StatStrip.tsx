import type { ReactElement } from "react";
import { useCallback } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import { isOn } from "@/domain/filters.ts";
import type { StatCell, StatFilter } from "@/domain/statStrip.ts";
import { cellDescription, compactWords, UNKNOWN } from "@/domain/statStrip.ts";
import { cn } from "@/lib/utils.ts";

interface StatStripProps {
  cells: readonly StatCell[];
  /** The current filters, to show which cell's filter is on. */
  query: EventQuery;
  /** Tapping a cell with a filter toggles it (DESIGN.md §10.1). */
  onToggle: (filter: StatFilter) => void;
}

function Number({ cell }: { cell: StatCell }): ReactElement {
  return (
    <span className={cn("font-mono text-stat-hero tabular-nums", cell.value === UNKNOWN ? "text-muted" : "text-ink")}>
      {cell.value}
    </span>
  );
}

function Words({ cell }: { cell: StatCell }): ReactElement {
  return <span className="text-micro text-muted uppercase">{cell.label}</span>;
}

interface FilterCellProps {
  cell: StatCell;
  filter: StatFilter;
  on: boolean;
  onToggle: (filter: StatFilter) => void;
}

/** A cell that filters: a toggle button with an accent underline when on (shape, not only colour). */
function FilterCell({ cell, filter, on, onToggle }: FilterCellProps): ReactElement {
  const toggle = useCallback(() => {
    onToggle(filter);
  }, [onToggle, filter]);
  return (
    <button
      type="button"
      aria-pressed={on}
      title={on ? "Shows every event again" : "Filters the map to these"}
      onClick={toggle}
      className="relative flex min-h-11 flex-col items-start rounded-sm pb-1.5 text-left hover:bg-ink/5 focus-visible:outline-2 focus-visible:outline-accent"
    >
      <Number cell={cell} />
      <Words cell={cell} />
      <span
        aria-hidden
        className={cn("absolute bottom-0 left-0 h-0.5 w-6 rounded-pill bg-accent", on ? "opacity-100" : "opacity-0")}
      />
    </button>
  );
}

/**
 * Numbers first (VIEWS.md `StatStrip`, DESIGN.md §10.1): observed events, high severity and
 * official warnings in view, the number in `statHero`, the words below. High severity and warnings
 * filter the map when tapped, and unfilter when tapped again.
 */
function StatStrip({ cells, query, onToggle }: StatStripProps): ReactElement {
  return (
    <div className="grid grid-cols-3 gap-3">
      {cells.map((cell) =>
        cell.filter === undefined ? (
          <p key={cell.label} className="flex flex-col items-start pb-1.5">
            <Number cell={cell} />
            <Words cell={cell} />
          </p>
        ) : (
          <FilterCell
            key={cell.label}
            cell={cell}
            filter={cell.filter}
            on={isOn(query, cell.filter)}
            onToggle={onToggle}
          />
        ),
      )}
    </div>
  );
}

/** "4 events · 1 high severity · 1 warning": the phone sheet's peek (owner, 2026-11-19). */
function CompactStatStrip({ cells }: { cells: readonly StatCell[] }): ReactElement {
  return (
    <p className="text-body text-muted">
      <span className="sr-only">{cells.map((cell) => cellDescription(cell)).join(", ")}</span>
      <span aria-hidden>
        {cells.map((cell, index) => (
          <span key={cell.label}>
            {index > 0 ? "  ·  " : ""}
            <span
              className={cn("font-mono font-semibold tabular-nums", cell.value === UNKNOWN ? "text-muted" : "text-ink")}
            >
              {cell.value}
            </span>
            {` ${compactWords(cell)}`}
          </span>
        ))}
      </span>
    </p>
  );
}

export { CompactStatStrip, StatStrip };

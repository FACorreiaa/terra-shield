import { ListFilter, X } from "lucide-react";
import type { ReactElement } from "react";
import { useCallback, useRef } from "react";

import type { ActiveFilter, EventQuery } from "@/domain/filters.ts";
import { activeFilters, filterLabel, removing } from "@/domain/filters.ts";
import { cn } from "@/lib/utils.ts";

import { FilterSheet } from "./FilterSheet.tsx";

interface FilterChipsProps {
  query: EventQuery;
  onChange: (query: EventQuery) => void;
}

/** The map's floating material, one row tall; an accent hairline when it carries a filter. */
const CHIP =
  "inline-flex h-11 shrink-0 items-center gap-1.5 rounded-pill border px-4 text-meta font-semibold text-ink shadow-floating material-floating hover:bg-ink/5 focus-visible:outline-2 focus-visible:outline-accent contrast-more:bg-panel contrast-more:backdrop-filter-none";

interface ChipProps extends FilterChipsProps {
  filter: ActiveFilter;
}

function RemovableChip({ filter, query, onChange }: ChipProps): ReactElement {
  const label = filterLabel(query, filter);
  const remove = useCallback(() => {
    onChange(removing(query, filter));
  }, [onChange, query, filter]);
  return (
    <button
      type="button"
      onClick={remove}
      aria-label={`Remove filter: ${label}`}
      className={cn(CHIP, "border-accent/60")}
    >
      {label}
      <X aria-hidden className="size-3" strokeWidth={3} />
    </button>
  );
}

/**
 * The filters on the map (VIEWS.md `FilterChips`, DESIGN.md §10.6): a Filters button that opens the
 * sheet, then one removable chip per filter that differs from the default. The row scrolls sideways
 * when it is wider than the space it has.
 */
function FilterChips({ query, onChange }: FilterChipsProps): ReactElement {
  const dialog = useRef<HTMLDialogElement>(null);
  const open = useCallback(() => {
    dialog.current?.showModal();
  }, []);
  const active = activeFilters(query);
  return (
    <>
      {/* Padded so focus rings and shadows aren't clipped by the scroller. */}
      <div className="-m-2 flex min-w-0 [scrollbar-width:none] gap-2 overflow-x-auto p-2">
        <button
          type="button"
          onClick={open}
          aria-haspopup="dialog"
          data-filters-button
          className={cn(CHIP, active.length === 0 ? "border-hairline-strong" : "border-accent/60")}
        >
          <ListFilter aria-hidden className="size-4" />
          Filters
          {active.length === 0 ? undefined : (
            <>
              <span className="rounded-pill bg-accent px-1.5 font-mono text-micro text-on-accent">{active.length}</span>
              <span className="sr-only"> on</span>
            </>
          )}
        </button>
        {active.map((filter) => (
          <RemovableChip key={filter} filter={filter} query={query} onChange={onChange} />
        ))}
      </div>
      <FilterSheet dialog={dialog} query={query} onChange={onChange} />
    </>
  );
}

export { FilterChips };

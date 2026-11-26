import type { ReactElement, RefObject } from "react";
import { useCallback } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import { DEFAULT_QUERY, isDefault } from "@/domain/filters.ts";

import { resetFilters, useSavedFilters } from "./filterPrefs.ts";
import { KindSection, SeveritySection, StatusSection, TypeSection, WindowSection } from "./FilterSections.tsx";

interface FilterSheetProps {
  dialog: RefObject<HTMLDialogElement | null>;
  query: EventQuery;
  onChange: (query: EventQuery) => void;
}

const HEADER_BUTTON =
  "min-h-11 rounded-sm px-3 text-body text-accent hover:bg-ink/5 focus-visible:outline-2 focus-visible:outline-accent";

/**
 * The filter sheet (DESIGN.md §10.6): hazard types, minimum severity, kinds, time window and status.
 * Changes apply as they are made and the map reloads behind it; "Reset" shows when anything differs
 * from the default. A native modal dialog: a bottom sheet on a phone, a panel on a wide screen.
 */
function FilterSheet({ dialog, query, onChange }: FilterSheetProps): ReactElement {
  const { showsDetections } = useSavedFilters();
  // As the apps: Reset also turns the heat layer off, and shows while it is on.
  const reset = useCallback(() => {
    resetFilters({ ...DEFAULT_QUERY, country: query.country });
  }, [query.country]);
  const done = useCallback(() => {
    dialog.current?.close();
  }, [dialog]);
  return (
    <dialog
      ref={dialog}
      aria-labelledby="filters-title"
      className="m-0 mt-auto max-h-[92dvh] w-full max-w-none rounded-t-lg border border-hairline-strong bg-panel p-0 text-ink shadow-floating backdrop:bg-canvas/60 lg:m-auto lg:mt-16 lg:max-h-[calc(100dvh-8rem)] lg:w-[min(30rem,calc(100%-2rem))] lg:rounded-lg"
    >
      <div className="sticky top-0 z-10 grid grid-cols-[1fr_auto_1fr] items-center border-b border-hairline bg-panel px-2 py-1">
        <span>
          {isDefault(query) && !showsDetections ? undefined : (
            <button type="button" onClick={reset} className={HEADER_BUTTON}>
              Reset
            </button>
          )}
        </span>
        <h2 id="filters-title" className="text-headline">
          Filters
        </h2>
        <span className="justify-self-end">
          <button type="button" onClick={done} className={`${HEADER_BUTTON} font-semibold`}>
            Done
          </button>
        </span>
      </div>
      <div className="flex flex-col gap-6 p-5">
        <TypeSection query={query} onChange={onChange} />
        <SeveritySection query={query} onChange={onChange} />
        <KindSection query={query} onChange={onChange} />
        <WindowSection query={query} onChange={onChange} />
        <StatusSection query={query} onChange={onChange} />
      </div>
    </dialog>
  );
}

export { FilterSheet };

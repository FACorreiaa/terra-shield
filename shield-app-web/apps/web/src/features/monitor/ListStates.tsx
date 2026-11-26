import type { ReactElement } from "react";

import { EmptyState } from "@/components/EmptyState.tsx";
import { emptyWords } from "@/domain/listNotes.ts";

const LINK_BUTTON =
  "min-h-11 rounded-sm text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent";
const SKELETON_ROWS = ["a", "b", "c", "d"];

/** *Loading:* the words, then rows in the list's shape (DESIGN.md §10.1). */
function LoadingRows(): ReactElement {
  return (
    <div aria-busy="true">
      <p className="px-4 pt-4 pb-2 text-body text-muted">Loading events…</p>
      <ul aria-hidden className="divide-y divide-ink/8">
        {SKELETON_ROWS.map((row) => (
          <li key={row} className="flex gap-3 px-4 py-3">
            <span className="size-5 shrink-0 rounded-full bg-raised/70" />
            <span className="flex flex-1 flex-col gap-2 motion-safe:animate-pulse">
              <span className="h-4 w-4/5 rounded-xs bg-raised/70" />
              <span className="h-3 w-1/2 rounded-xs bg-raised/50" />
              <span className="h-5 w-24 rounded-xs bg-raised/50" />
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Nothing has arrived and the last request failed, or the browser is offline: say so, with Retry. */
function NothingLoaded({ message, onRetry }: { message: string; onRetry: () => void }): ReactElement {
  return (
    <div className="flex flex-col items-start gap-1 px-4 py-4">
      <p className="text-body text-danger">{message}</p>
      <button type="button" onClick={onRetry} className={LINK_BUTTON}>
        Retry
      </button>
    </div>
  );
}

interface EmptyViewProps {
  filtered: boolean;
  onReset: () => void;
  onZoomOut: () => void;
}

/** *Empty view:* says what isn't covered instead of implying safety (DECISIONS #7). */
function EmptyView({ filtered, onReset, onZoomOut }: EmptyViewProps): ReactElement {
  const words = emptyWords(filtered);
  return (
    <EmptyState>
      <p className="text-body text-ink">{words.title}</p>
      <p className="text-meta text-muted">{words.note}</p>
      <div className="flex flex-wrap gap-x-4">
        {filtered ? (
          <button type="button" onClick={onReset} className={LINK_BUTTON}>
            Reset filters
          </button>
        ) : undefined}
        <button type="button" onClick={onZoomOut} className={LINK_BUTTON}>
          Zoom out
        </button>
      </div>
    </EmptyState>
  );
}

export { EmptyView, LoadingRows, NothingLoaded };

import { X } from "lucide-react";
import type { ReactElement } from "react";

import { Banner } from "./DetailBanner.tsx";
import type { DetailState } from "./detailState.ts";

const WORDS = {
  loading: "Loading event…",
  failed: "This event couldn’t be loaded. Check your connection and try again.",
} as const;

/**
 * The detail before any copy of the event exists: a shared link still loading, one that failed,
 * or one to an event its source no longer publishes (never seen here, so there is nothing to show
 * but that). The apps drop such a selection; the web keeps it, since links are how people arrive.
 */
interface DetailPlaceholderProps {
  state: DetailState;
  onClose: () => void;
  onRetry: () => void;
}

function DetailPlaceholder({ state, onClose, onRetry }: DetailPlaceholderProps): ReactElement {
  return (
    <article aria-label="Event" aria-busy={state.kind === "loading"} className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <h2 tabIndex={-1} data-detail-heading className="pt-2 text-title text-ink outline-none">
          Event
        </h2>
        <button
          type="button"
          aria-label="Close"
          title="Close (Esc)"
          onClick={onClose}
          className="-mr-2 flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>
      {state.kind === "gone" || state.kind === "loaded" ? (
        <Banner tone="muted">This event is no longer published by its source.</Banner>
      ) : (
        <p className="text-meta text-muted">{WORDS[state.kind]}</p>
      )}
      {state.kind === "failed" ? (
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex min-h-11 items-center self-start rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
        >
          Try again
        </button>
      ) : undefined}
    </article>
  );
}

export { DetailPlaceholder };

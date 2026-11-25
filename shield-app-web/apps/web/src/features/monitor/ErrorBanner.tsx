import { TriangleAlert } from "lucide-react";
import type { ReactElement } from "react";

/** *Error:* `danger` text on `panel`, with Retry; the map stays as it was (DESIGN.md §10.1). */
function ErrorBanner({ message, onRetry }: { message: string; onRetry: () => void }): ReactElement {
  return (
    <div
      role="alert"
      className="flex w-full max-w-[35rem] items-center gap-3 rounded-md border border-danger/35 bg-panel py-1 pr-1 pl-3 shadow-floating"
    >
      <TriangleAlert
        aria-hidden
        className="size-4 shrink-0 text-danger"
        fill="currentColor"
        stroke="var(--color-panel)"
      />
      <p className="flex-1 text-body text-danger">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="min-h-11 shrink-0 rounded-sm px-3 text-body font-semibold text-accent hover:bg-ink/5 focus-visible:outline-2 focus-visible:outline-accent"
      >
        Retry
      </button>
    </div>
  );
}

export { ErrorBanner };

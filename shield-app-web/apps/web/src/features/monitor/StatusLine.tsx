import type { ReactElement } from "react";

import type { LiveStatus, Tone } from "@/domain/liveStatus.ts";
import { cn } from "@/lib/utils.ts";

const DOT: Readonly<Record<Tone, string>> = {
  ok: "border-ok",
  warning: "border-warning",
  danger: "border-danger",
  muted: "border-muted",
};

interface StatusLineProps {
  status: LiveStatus;
  onRetry: () => void;
}

/**
 * The live dot and how fresh the map's data is (DESIGN.md §10.1 states). The dot is a ring while the
 * map only polls; it fills once the live stream arrives (W6), so shape carries it as well as colour.
 */
function StatusLine({ status, onRetry }: StatusLineProps): ReactElement {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span aria-hidden className={cn("size-1.5 shrink-0 rounded-full border-[1.5px]", DOT[status.dot])} />
      <span className={cn("font-mono text-meta", status.warns ? "text-warning" : "text-muted")}>{status.text}</span>
      {status.retry ? (
        <button
          type="button"
          onClick={onRetry}
          className="-my-3 min-h-11 shrink-0 px-2 text-meta font-semibold text-accent focus-visible:outline-2 focus-visible:outline-accent"
        >
          Retry
        </button>
      ) : undefined}
    </span>
  );
}

/** A thin accent line while a request is in flight (DESIGN.md §10.1 *Loading*); steady under reduced motion. */
function ProgressLine({ active }: { active: boolean }): ReactElement | undefined {
  if (!active) {
    return undefined;
  }
  return (
    <span aria-hidden className="absolute inset-x-3 bottom-0 h-0.5 overflow-hidden rounded-pill">
      <span className="block h-full w-1/3 animate-[progress_1.2s_ease-in-out_infinite] rounded-pill bg-accent motion-reduce:w-full motion-reduce:animate-none motion-reduce:opacity-60" />
    </span>
  );
}

export { ProgressLine, StatusLine };

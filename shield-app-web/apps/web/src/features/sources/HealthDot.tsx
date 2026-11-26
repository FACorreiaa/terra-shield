import type { ReactElement } from "react";

import type { SourceHealth } from "@/domain/sources.ts";

const DOT: Readonly<Record<SourceHealth, string>> = { ok: "bg-ok", warning: "bg-warning", danger: "bg-danger" };

/** What the dot says, for screen readers: colour is never the only signal (DESIGN.md §14). */
const WORDS: Readonly<Record<SourceHealth, string>> = {
  ok: "Healthy",
  warning: "Last check failed",
  danger: "Stale",
};

/** `ok` / `warning` / `danger` (DESIGN.md §10.7), 8 px, with its words for screen readers. */
function HealthDot({ health }: { health: SourceHealth }): ReactElement {
  return (
    <>
      <span aria-hidden className={`inline-block size-2 shrink-0 rounded-full ${DOT[health]}`} />
      <span className="sr-only">{WORDS[health]}</span>
    </>
  );
}

export { HealthDot };

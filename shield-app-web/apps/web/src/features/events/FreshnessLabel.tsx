import type { ReactElement } from "react";

import { relative, utcClock } from "./freshness.ts";

/** "Updated 12 min ago · 14:02 UTC" (DESIGN.md §2). */
function FreshnessLabel({ updatedAt, now }: { updatedAt: string; now: Date }): ReactElement {
  const date = new Date(updatedAt);
  return (
    <time dateTime={updatedAt} className="font-mono text-meta text-muted">
      Updated {relative(date, now)} · {utcClock(date)}
    </time>
  );
}

export { FreshnessLabel };

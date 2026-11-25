import type { HazardEvent } from "@shield/api-client";
import { CircleCheck, ClockAlert } from "lucide-react";
import type { ReactElement } from "react";

import { relative, resolvedWords, utcClock } from "@/domain/freshness.ts";

/** "Updated 12 min ago · 14:02 UTC" (DESIGN.md §2). */
function FreshnessLabel({ updatedAt, now }: { updatedAt: string; now: Date }): ReactElement {
  const date = new Date(updatedAt);
  return (
    <time dateTime={updatedAt} className="font-mono text-meta text-muted">
      Updated {relative(date, now)} · {utcClock(date)}
    </time>
  );
}

interface RowStatusProps {
  event: HazardEvent;
  now: Date;
  /** When the event's source last answered, once it is stale; undefined while it is fresh. */
  staleSince: string | undefined;
}

/**
 * A list row's last line (FreshnessLabel.swift): "Resolved · ended 3 h ago" in `ok`, "Stale · last
 * update 3 h ago" in `warning` with a clock, else its freshness. Words, never colour alone.
 */
function RowStatus({ event, now, staleSince }: RowStatusProps): ReactElement {
  const resolved = resolvedWords(event, now);
  if (resolved !== undefined) {
    return (
      <span className="flex items-center gap-1.5 font-mono text-meta text-ok">
        <CircleCheck aria-hidden className="size-3.5" />
        {resolved}
      </span>
    );
  }
  if (staleSince !== undefined) {
    return (
      <span className="flex items-center gap-1.5 font-mono text-meta text-warning">
        <ClockAlert aria-hidden className="size-3.5" />
        {`Stale · last update ${relative(new Date(staleSince), now)}`}
      </span>
    );
  }
  return <FreshnessLabel updatedAt={event.updatedAt} now={now} />;
}

export { FreshnessLabel, RowStatus };

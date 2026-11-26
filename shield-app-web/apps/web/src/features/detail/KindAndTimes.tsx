import type { HazardEvent } from "@shield/api-client";
import { CircleCheck } from "lucide-react";
import type { ReactElement } from "react";

import { detailTimes } from "@/domain/eventFacts.ts";
import { resolvedWords, stamp, utcDay } from "@/domain/freshness.ts";
import { KindChip } from "@/features/events/KindChip.tsx";

interface KindAndTimesProps {
  event: HazardEvent;
  gone: boolean;
  now: Date;
}

/** Not one of the four hazard kinds: a research estimate (docs/attrs.md, UCDP). */
const CONFLICT_KIND = "Monthly estimate (UCDP)";

function Status({ event, gone, now }: KindAndTimesProps): ReactElement | undefined {
  const resolved = resolvedWords(event, now);
  if (resolved !== undefined) {
    return (
      <span className="flex items-center gap-1 font-mono text-meta text-ok">
        <CircleCheck className="size-3.5" aria-hidden />
        {resolved}
      </span>
    );
  }
  // A conflict is "past 90 days", never live (BACKLOG L2).
  if (gone || event.type === "conflict") {
    return undefined;
  }
  return <span className="text-meta text-muted">Active</span>;
}

/** Kind, status and the event's times (DESIGN.md §10.3 item 3): "Started 3 h ago · 11:02 UTC". */
function KindAndTimes({ event, gone, now }: KindAndTimesProps): ReactElement {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {event.type === "conflict" ? (
          <span className="text-meta font-semibold text-ink">{CONFLICT_KIND}</span>
        ) : (
          <KindChip kind={event.kind} />
        )}
        <Status event={event} gone={gone} now={now} />
      </div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-meta">
        {detailTimes(event, now).map((time) => (
          <div key={time.label} className="contents">
            <dt className="text-muted">{time.label}</dt>
            <dd className="font-mono text-ink tabular-nums">
              {time.dateOnly ? utcDay(time.date) : stamp(time.date, now)}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

export { KindAndTimes };

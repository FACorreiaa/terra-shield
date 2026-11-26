import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";
import { use } from "react";

import { conflictDateLine, isApproximateLocation } from "@/domain/conflictFacts.ts";
import { CONFLICT } from "@/domain/taxonomy.ts";

import { RowStatus } from "./FreshnessLabel.tsx";
import { KindChip } from "./KindChip.tsx";
import { SourceLookupContext } from "./sourceLookup.ts";

interface RowFactsProps {
  event: HazardEvent;
  now: Date;
  /** When the event's source last succeeded, if it is stale now. */
  staleSince: string | undefined;
}

/**
 * A row's lines under its badge (EventRow.swift): a hazard's kind, source and freshness, or a
 * conflict's dates. A conflict is a monthly estimate, never its kind or "updated N ago".
 */
function RowFacts({ event, now, staleSince }: RowFactsProps): ReactElement {
  const { byId } = use(SourceLookupContext);
  if (event.type === CONFLICT) {
    return (
      <span className="text-meta text-muted">
        {conflictDateLine(event)}
        {/* The dashed ring's words, as the apps' marker label says them (MapAnnotations.swift). */}
        {isApproximateLocation(event) ? <span className="sr-only">, approximate location</span> : undefined}
      </span>
    );
  }
  return (
    <>
      <span className="flex flex-wrap items-center gap-2">
        <KindChip kind={event.kind} />
        <span className="text-meta text-muted">{byId.get(event.source)?.name ?? event.source.toUpperCase()}</span>
      </span>
      <RowStatus event={event} now={now} staleSince={staleSince} />
    </>
  );
}

export { RowFacts };

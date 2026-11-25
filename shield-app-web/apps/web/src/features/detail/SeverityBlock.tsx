import type { HazardEvent } from "@shield/api-client";
import { Flame, Ruler } from "lucide-react";
import type { ReactElement } from "react";

import { isPrescribed, severityNote, severityRating, showsProvisional } from "@/domain/eventFacts.ts";
import { SeverityBadge } from "@/features/events/SeverityBadge.tsx";

import { Eyebrow } from "./Eyebrow.tsx";

/**
 * `SEVERITY · AEGIS / SOURCE` (DESIGN.md §4): the app's badge beside the source's value, verbatim
 * and uncoloured, then what the level rests on and why it reads as it does.
 */
function SeverityBlock({ event }: { event: HazardEvent }): ReactElement {
  const rating = severityRating(event);
  const note = severityNote(event);
  const RatingIcon = isPrescribed(event) ? Flame : Ruler;
  return (
    <section aria-label="Severity" className="flex flex-col gap-2">
      <Eyebrow>Severity · Aegis / source</Eyebrow>
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
        <SeverityBadge severity={event.severity} provisional={showsProvisional(event)} />
        {event.sourceSeverity === undefined || event.sourceSeverity === "" ? undefined : (
          <p className="min-w-0 text-meta text-muted">
            <span className="sr-only">Source says </span>
            {event.sourceSeverity}
          </p>
        )}
      </div>
      {rating === undefined ? undefined : (
        <p className="flex items-center gap-1.5 text-meta font-semibold text-ink">
          <RatingIcon className="size-3.5 shrink-0" aria-hidden />
          {rating}
        </p>
      )}
      {note === undefined ? undefined : <p className="text-meta text-muted">{note}</p>}
    </section>
  );
}

export { SeverityBlock };

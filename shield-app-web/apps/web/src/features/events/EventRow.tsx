import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";
import { use, useCallback } from "react";

import { Icon } from "@/design/icons.ts";
import { hazardTypeTokens } from "@/design/tokens.gen.ts";
import { CONFLICT, eventType } from "@/domain/taxonomy.ts";
import { cn } from "@/lib/utils.ts";

import { RowFacts } from "./RowFacts.tsx";
import { SeverityBadge } from "./SeverityBadge.tsx";
import { SourceLookupContext } from "./sourceLookup.ts";

interface EventRowProps {
  event: HazardEvent;
  selected: boolean;
  onSelect: (id: string) => void;
  now: Date;
  /** Where, in words: the event's countries on the map, "Near Chile, 38 km offshore" or nothing in a country. */
  where: string | undefined;
}

function EventRow({ event, selected, onSelect, now, where }: EventRowProps): ReactElement {
  const typeToken = hazardTypeTokens[eventType(event.type)];
  // A stale source's row fades like its marker and says so (EventRow.swift); the source by name.
  const { byId, stale: staleSources } = use(SourceLookupContext);
  const source = byId.get(event.source);
  // A conflict is a monthly estimate, never "stale" (EventRow.swift).
  const conflict = event.type === CONFLICT;
  const stale = !conflict && staleSources.has(event.source);
  const select = useCallback(() => {
    onSelect(event.id);
  }, [onSelect, event.id]);
  return (
    <li data-event-id={event.id}>
      <button
        type="button"
        onClick={select}
        aria-current={selected ? "true" : undefined}
        className={cn("flex w-full gap-3 px-4 py-3 text-left hover:bg-raised/60", selected && "bg-raised")}
      >
        <Icon name={typeToken.icon} aria-label={typeToken.label} className="mt-0.5 size-5 shrink-0 text-muted" />
        <span className="flex min-w-0 flex-col gap-1.5">
          <span className={cn("text-headline", stale || event.status === "resolved" ? "text-muted" : "text-ink")}>
            {event.title}
          </span>
          {where === undefined ? undefined : <span className="text-meta text-muted">{where}</span>}
          <SeverityBadge
            severity={event.severity}
            sourceSeverity={event.sourceSeverity}
            provisional={event.severityProvisional}
          />
          <RowFacts
            event={event}
            now={now}
            staleSince={stale ? (source?.lastSuccessAt ?? event.fetchedAt) : undefined}
          />
        </span>
      </button>
    </li>
  );
}

export { EventRow };

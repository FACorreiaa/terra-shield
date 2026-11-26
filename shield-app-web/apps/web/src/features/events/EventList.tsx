import type { HazardEvent } from "@shield/api-client";
import { hazardType } from "@shield/api-client";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { Icon } from "@/design/icons.ts";
import { hazardTypeTokens } from "@/design/tokens.gen.ts";
import { place } from "@/domain/place.ts";
import { cn } from "@/lib/utils.ts";

import { FreshnessLabel } from "./FreshnessLabel.tsx";
import { KindChip } from "./KindChip.tsx";
import { SeverityBadge } from "./SeverityBadge.tsx";

interface EventRowProps {
  event: HazardEvent;
  selected: boolean;
  onSelect: (id: string) => void;
  now: Date;
}

function EventRow({ event, selected, onSelect, now }: EventRowProps): ReactElement {
  const typeToken = hazardTypeTokens[hazardType(event.type)];
  const where = place(event);
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
          <span className="text-headline text-ink">{event.title}</span>
          {where === undefined ? undefined : <span className="text-meta text-muted">{where}</span>}
          <SeverityBadge
            severity={event.severity}
            sourceSeverity={event.sourceSeverity}
            provisional={event.severityProvisional}
          />
          <span className="flex flex-wrap items-center gap-2">
            <KindChip kind={event.kind} />
            <span className="text-meta text-muted">{event.source.toUpperCase()}</span>
          </span>
          <FreshnessLabel updatedAt={event.updatedAt} now={now} />
        </span>
      </button>
    </li>
  );
}

interface EventListProps {
  events: readonly HazardEvent[];
  selectedId: string | undefined;
  onSelect: (id: string) => void;
  now: Date;
}

/** Events in display order (the server sorts: most severe, then most recently updated). */
function EventList({ events, selectedId, onSelect, now }: EventListProps): ReactElement {
  return (
    <ul className="divide-y divide-ink/8">
      {events.map((event) => (
        <EventRow key={event.id} event={event} selected={selectedId === event.id} onSelect={onSelect} now={now} />
      ))}
    </ul>
  );
}

export { EventList, EventRow };

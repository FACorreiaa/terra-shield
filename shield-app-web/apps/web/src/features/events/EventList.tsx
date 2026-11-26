import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";

import { place } from "@/domain/place.ts";

import { EventRow } from "./EventRow.tsx";

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
        <EventRow
          key={event.id}
          event={event}
          selected={selectedId === event.id}
          onSelect={onSelect}
          now={now}
          where={place(event)}
        />
      ))}
    </ul>
  );
}

export { EventList };

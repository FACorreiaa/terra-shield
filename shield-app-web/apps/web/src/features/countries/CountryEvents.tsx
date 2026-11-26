import type { HazardEvent } from "@shield/api-client";
import { X } from "lucide-react";
import type { ReactElement } from "react";
import { useCallback } from "react";

import type { CountrySummary, MatrixCell } from "@/domain/countries.ts";
import { eventsIn } from "@/domain/countries.ts";
import { capNote, cellWords } from "@/domain/countryOverview.ts";
import { nearbyWords } from "@/domain/place.ts";
import { EventRow } from "@/features/events/EventRow.tsx";

import { Section } from "./Section.tsx";

interface CountryEventsProps {
  summary: CountrySummary;
  cell: MatrixCell | undefined;
  onClearCell: () => void;
  selectedId: string | undefined;
  onSelect: (event: HazardEvent) => void;
  now: Date;
}

/**
 * The country's events (DESIGN.md §10.2 item 6), all of them whatever the map's filters, or the
 * chosen cell's, with "Near Chile, 38 km offshore" for where.
 */
function CountryEvents({ summary, cell, onClearCell, selectedId, onSelect, now }: CountryEventsProps): ReactElement {
  const { iso2, name } = summary.country;
  const listed = eventsIn(summary.events, cell);
  const note = capNote(summary, listed.length, cell);
  const select = useCallback(
    (id: string) => {
      const event = summary.events.find((candidate) => candidate.id === id);
      if (event !== undefined) {
        onSelect(event);
      }
    },
    [summary.events, onSelect],
  );
  return (
    <Section title="Events">
      {cell === undefined ? undefined : (
        <button
          type="button"
          onClick={onClearCell}
          aria-label={`Remove filter: ${cellWords(cell).replace(" · ", ", ")}`}
          className="inline-flex min-h-11 items-center gap-1.5 self-start rounded-full border border-accent/60 px-3 text-meta font-semibold text-ink hover:bg-ink/8 focus-visible:outline-2 focus-visible:outline-accent"
        >
          {cellWords(cell)}
          <X aria-hidden className="size-3" strokeWidth={3} />
        </button>
      )}
      {note === undefined ? undefined : <p className="text-meta text-muted">{note}</p>}
      <ul className="-mx-4 divide-y divide-ink/8">
        {listed.map((event) => (
          <EventRow
            key={event.id}
            event={event}
            selected={selectedId === event.id}
            onSelect={select}
            now={now}
            where={nearbyWords(event, iso2, name)}
          />
        ))}
      </ul>
    </Section>
  );
}

export { CountryEvents };

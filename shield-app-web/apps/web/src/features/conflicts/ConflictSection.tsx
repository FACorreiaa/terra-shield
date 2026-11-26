import type { ReactElement } from "react";
import { useId } from "react";

import { CONFLICT_SECTION, conflictCount, conflictSectionNote } from "@/domain/conflicts.ts";
import { EventList } from "@/features/events/EventList.tsx";

import type { Conflicts } from "./useConflicts.ts";

interface ConflictSectionProps {
  conflicts: Conflicts;
  selectedId: string | undefined;
  onSelect: (id: string) => void;
  now: Date;
  /** The view has no hazards: the section says so first (EventList.swift). */
  noHazards?: boolean;
}

/**
 * Armed conflicts in the list (`conflictSection`, EventList.swift): their own section after the
 * hazards, never mixed into them, saying what the data are and how many the map grouped.
 */
function ConflictSection(props: ConflictSectionProps): ReactElement | undefined {
  const { conflicts, selectedId, onSelect, now, noHazards = false } = props;
  const titleId = useId();
  const { list, events } = conflicts;
  if (list === undefined) {
    return undefined;
  }
  const clustered = conflictCount(list) - list.events.length;
  return (
    <>
      {noHazards ? (
        <p className="px-4 py-3 text-body text-muted">No hazards in this view from covered sources.</p>
      ) : undefined}
      <section aria-labelledby={titleId} data-conflict-section className="border-t border-hairline">
        <header className="flex flex-col gap-1 px-4 pt-4 pb-2">
          <h2 id={titleId} className="text-micro tracking-[0.08em] text-muted uppercase">
            {CONFLICT_SECTION}
          </h2>
          <p className="text-meta text-muted">{conflictSectionNote(clustered)}</p>
        </header>
        <EventList events={events} selectedId={selectedId} onSelect={onSelect} now={now} />
      </section>
    </>
  );
}

export { ConflictSection };

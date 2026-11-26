import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";
import { useCallback, useState } from "react";

import type { RecentChange } from "@/domain/changes.ts";
import { changeSentence } from "@/domain/changes.ts";
import { relative } from "@/domain/freshness.ts";

import { Section } from "./Section.tsx";

/** Changes beyond this many wait behind "Show all". */
const SHOWN = 4;

interface ChangeRowProps {
  change: RecentChange;
  /** The listed event the change is about; a capped list may not hold it. */
  event: HazardEvent | undefined;
  now: Date;
  onSelect: (event: HazardEvent) => void;
}

function ChangeRow({ change, event, now, onSelect }: ChangeRowProps): ReactElement {
  const select = useCallback(() => {
    if (event !== undefined) {
      onSelect(event);
    }
  }, [event, onSelect]);
  const words = (
    <>
      <span className="text-body text-ink">{change.title}</span>
      <span className="text-meta text-muted">{`${changeSentence(change.changes)} · ${relative(change.at, now)}`}</span>
    </>
  );
  const box = "flex w-full flex-col gap-0.5 py-2 text-left";
  return (
    <li>
      {event === undefined ? (
        <p className={box}>{words}</p>
      ) : (
        <button
          type="button"
          onClick={select}
          className={`${box} rounded-sm hover:bg-raised/60 focus-visible:outline-2 focus-visible:outline-accent`}
        >
          {words}
        </button>
      )}
    </li>
  );
}

interface RecentChangesProps {
  changes: readonly RecentChange[];
  events: readonly HazardEvent[];
  now: Date;
  onSelect: (event: HazardEvent) => void;
}

/** What changed in the last 24 h, newest first (DESIGN.md §10.2 item 5); a row opens its event. */
function RecentChanges({ changes, events, now, onSelect }: RecentChangesProps): ReactElement | undefined {
  const [all, setAll] = useState(false);
  const toggle = useCallback(() => {
    setAll((was) => !was);
  }, []);
  if (changes.length === 0) {
    return undefined;
  }
  const shown = all ? changes : changes.slice(0, SHOWN);
  return (
    <Section title="Changed in the last 24 h">
      <ul className="flex flex-col">
        {shown.map((change) => (
          <ChangeRow
            key={`${change.eventId} ${change.at.toISOString()}`}
            change={change}
            event={events.find((event) => event.id === change.eventId)}
            now={now}
            onSelect={onSelect}
          />
        ))}
      </ul>
      {changes.length > SHOWN ? (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={all}
          className="inline-flex min-h-11 items-center self-start rounded-sm px-1 text-meta font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
        >
          {all ? "Show fewer" : `Show all ${changes.length}`}
        </button>
      ) : undefined}
    </Section>
  );
}

export { RecentChanges };

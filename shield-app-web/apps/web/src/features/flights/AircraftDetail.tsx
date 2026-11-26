import type { Aircraft, AircraftList } from "@shield/api-client";
import { X } from "lucide-react";
import type { ReactElement } from "react";

import { aircraftFigures, aircraftNotes, aircraftTitle, flightsAttribution } from "@/domain/aircraft.ts";
import { cn } from "@/lib/utils.ts";

import { useTick } from "./useTick.ts";

interface AircraftDetailProps {
  aircraft: Aircraft;
  /** The answer it came from, for the attribution and its licence link. */
  list: AircraftList | undefined;
  onClose: () => void;
}

function Notes({ aircraft, now }: { aircraft: Aircraft; now: Date }): ReactElement {
  return (
    <div className="flex flex-col gap-1">
      {aircraftNotes(aircraft, now).map((note) => {
        const emergency = note.startsWith("Emergency");
        return (
          <p key={note} className={cn("text-meta", emergency ? "font-semibold text-warning" : "text-muted")}>
            {note}
          </p>
        );
      })}
    </div>
  );
}

/**
 * The selected aircraft in the inspector or the sheet (`AircraftDetailView`, Flights.swift): a
 * position report, never a flight page, so no route and no history (ADR 28).
 */
function AircraftDetail({ aircraft, list, onClose }: AircraftDetailProps): ReactElement {
  const now = useTick();
  const title = aircraftTitle(aircraft);
  return (
    <article aria-label={title} className="flex flex-col gap-5 p-4">
      <header className="flex items-start gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <p className="text-micro tracking-[0.08em] text-muted uppercase">Aircraft</p>
          <h2 tabIndex={-1} data-detail-heading className="text-title text-ink outline-none">
            {title}
          </h2>
        </div>
        <button
          type="button"
          aria-label="Close"
          title="Close (Esc)"
          onClick={onClose}
          className="-mt-2 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
        >
          <X className="size-4" aria-hidden />
        </button>
      </header>
      <Notes aircraft={aircraft} now={now} />
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
        {aircraftFigures(aircraft).map((figure) => (
          <div key={figure.label} className="contents">
            <dt className="text-meta text-muted">{figure.label}</dt>
            <dd className="font-mono text-body text-ink tabular-nums select-text">{figure.value}</dd>
          </div>
        ))}
      </dl>
      <footer className="flex flex-col gap-1 border-t border-hairline pt-4">
        <p className="text-meta text-muted">
          Live positions from adsb.lol, a community network of receivers. Not every aircraft is covered, and positions
          can be minutes old.
        </p>
        {list?.licenseUrl === undefined ? (
          <p className="text-meta text-ink">{flightsAttribution(list)}</p>
        ) : (
          <a
            href={list.licenseUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-meta text-accent underline-offset-2 hover:underline"
          >
            {flightsAttribution(list)}
          </a>
        )}
      </footer>
    </article>
  );
}

export { AircraftDetail };

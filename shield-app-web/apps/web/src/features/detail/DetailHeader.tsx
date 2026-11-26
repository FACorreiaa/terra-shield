import type { HazardEvent } from "@shield/api-client";
import { MapPin, X } from "lucide-react";
import type { ReactElement } from "react";

import { locationNote } from "@/domain/eventFacts.ts";
import { place } from "@/domain/place.ts";
import type { MarkerState } from "@/features/map/markerArt.ts";

import { EventGlyph } from "./EventGlyph.tsx";

/** DESIGN.md §12: the types that get a background behind the detail's header; the rest keep the plain panel. */
const HEADER_ART: Readonly<Record<string, string>> = {
  storm: "/brand/precipitation.svg",
  tropical_cyclone: "/brand/precipitation.svg",
  heavy_rain: "/brand/precipitation.svg",
  flood: "/brand/precipitation.svg",
  wind: "/brand/precipitation.svg",
  wildfire: "/brand/wildfire.svg",
  heat: "/brand/wildfire.svg",
  drought: "/brand/wildfire.svg",
  volcano: "/brand/wildfire.svg",
};

interface DetailHeaderProps {
  event: HazardEvent;
  stale: boolean;
  gone: boolean;
  onClose: () => void;
}

/**
 * The type's art at 30%, bled past the padding and faded out radially to transparent (§12's
 * vignette), so it melts into the sheet or the inspector alike. Decoration only.
 */
function HeaderArt({ type, faded }: { type: string; faded: boolean }): ReactElement | undefined {
  const art = HEADER_ART[type];
  if (art === undefined) {
    return undefined;
  }
  return (
    <img
      src={art}
      alt=""
      className={`pointer-events-none absolute -inset-4 -z-10 size-[calc(100%+2rem)] [mask-image:radial-gradient(ellipse_at_80%_40%,black,transparent_70%)] object-cover opacity-30 ${faded ? "grayscale" : ""}`}
    />
  );
}

/** What and where (DESIGN.md §10.3 item 1): the marker, the title, every country, the location note, Close. */
function markerState(event: HazardEvent, stale: boolean): MarkerState {
  if (event.status === "resolved") {
    return "resolved";
  }
  return stale ? "stale" : "live";
}

function DetailHeader({ event, stale, gone, onClose }: DetailHeaderProps): ReactElement {
  const where = place(event);
  const note = locationNote(event);
  const muted = event.status === "resolved" || gone;
  return (
    <header className="relative isolate flex items-start gap-3">
      <HeaderArt type={event.type} faded={muted} />
      <EventGlyph
        event={event}
        state={gone ? "live" : markerState(event, stale)}
        gone={gone}
        className="size-11 shrink-0"
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2
          tabIndex={-1}
          data-detail-heading
          className={`text-title outline-none ${muted ? "text-muted" : "text-ink"}`}
        >
          {event.title}
        </h2>
        {where === undefined ? undefined : (
          <p className="flex items-start gap-1.5 text-meta text-muted">
            <MapPin className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            {where}
          </p>
        )}
        {note === undefined ? undefined : <p className="text-meta text-muted">{note}</p>}
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
  );
}

export { DetailHeader };

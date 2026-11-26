import type { HazardEvent } from "@shield/api-client";
import { X } from "lucide-react";
import type { FocusEvent, ReactElement } from "react";
import { useCallback, useEffect, useRef, useState } from "react";

import { arrivalLine } from "@/domain/eventFacts.ts";
import { isMarker } from "@/features/map/eventLayer.ts";
import { MarkerImage } from "@/features/map/MarkerImage.tsx";
import { cn } from "@/lib/utils.ts";

/** The edge in the event's severity colour (NewEventToast.swift: 60 %). */
const BORDER: Readonly<Record<HazardEvent["severity"], string>> = {
  minor: "border-hairline-strong",
  moderate: "border-hairline-strong",
  severe: "border-severity-severe/60",
  extreme: "border-severity-extreme/60",
};

interface NewEventToastProps {
  event: HazardEvent;
  onOpen: (event: HazardEvent) => void;
  onDismiss: () => void;
  /** Pointer or keyboard focus on the toast (true) or gone from it: its clock waits meanwhile. */
  onHold: (held: boolean) => void;
  /** Focus moves to it on arrival: it replaced a toast that had focus. */
  takeFocus: boolean;
}

/** Whether focus leaves `container` for somewhere outside it. */
function leaving(event: FocusEvent<HTMLElement>): boolean {
  return !(event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget));
}

/**
 * One line at the top of the map for a severe or extreme live arrival in view (DESIGN.md §10.1,
 * NewEventToast.swift): "New · Severe earthquake · M6.1 · Chile". Pressing it opens the event; no
 * sound. It slides in (fades only under reduced motion) and waits while pointed at or focused.
 * Screen readers hear it from `ToastAnnouncer`, which stays mounted so the line is read out.
 */
function NewEventToast({ event, onOpen, onDismiss, onHold, takeFocus }: NewEventToastProps): ReactElement {
  const line = arrivalLine(event);
  const button = useRef<HTMLButtonElement>(null);
  const open = useCallback(() => {
    onOpen(event);
  }, [event, onOpen]);
  // Held while either the pointer or focus is on it: leaving with one doesn't let go of the other.
  const inside = useRef({ pointer: false, focus: false });
  const tell = useCallback(() => {
    onHold(inside.current.pointer || inside.current.focus);
  }, [onHold]);
  const enter = useCallback(() => {
    inside.current.pointer = true;
    tell();
  }, [tell]);
  const exit = useCallback(() => {
    inside.current.pointer = false;
    tell();
  }, [tell]);
  const focus = useCallback(() => {
    inside.current.focus = true;
    tell();
  }, [tell]);
  const blur = useCallback(
    (moved: FocusEvent<HTMLElement>) => {
      if (leaving(moved)) {
        inside.current.focus = false;
        tell();
      }
    },
    [tell],
  );
  useEffect(() => {
    if (takeFocus) {
      button.current?.focus();
    }
  }, [takeFocus]);
  return (
    <div
      onPointerEnter={enter}
      onPointerLeave={exit}
      onFocus={focus}
      onBlur={blur}
      className={cn(
        "toast-in flex min-h-11 w-full max-w-[35rem] items-center gap-1 rounded-md border py-0.5 pr-0.5 pl-1 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none",
        BORDER[event.severity],
      )}
    >
      <button
        ref={button}
        type="button"
        onClick={open}
        className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 text-left focus-visible:outline-2 focus-visible:outline-accent"
      >
        {isMarker(event) ? <MarkerImage event={event} className="size-5 shrink-0" /> : undefined}
        <span className="min-w-0 flex-1 truncate text-body font-semibold text-ink">{line}</span>
        <span className="sr-only">, opens the event</span>
      </button>
      <button
        type="button"
        onClick={onDismiss}
        aria-label="Dismiss"
        className="flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
      >
        <X aria-hidden className="size-4" />
      </button>
    </div>
  );
}

/** Long enough after clearing that screen readers hear the line as new, even the same words again. */
const REANNOUNCE_MS = 100;

/**
 * A polite live region, always in the page, that reads each new toast's line once: cleared, then
 * set, so two toasts with the same words are both heard. Re-renders change nothing, so say nothing.
 */
function ToastAnnouncer({ event }: { event: HazardEvent | undefined }): ReactElement {
  const [said, setSaid] = useState("");
  useEffect(() => {
    if (event === undefined) {
      return undefined;
    }
    const clearing = requestAnimationFrame(() => {
      setSaid("");
    });
    const saying = setTimeout(() => {
      setSaid(arrivalLine(event));
    }, REANNOUNCE_MS);
    return () => {
      cancelAnimationFrame(clearing);
      clearTimeout(saying);
    };
  }, [event]);
  return <output className="sr-only">{said}</output>;
}

export { NewEventToast, ToastAnnouncer };

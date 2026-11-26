import type { HazardEvent } from "@shield/api-client";
import { useCallback, useEffect, useMemo, useState } from "react";

import { isAtLeast } from "@/domain/taxonomy.ts";

/** A toast is read out once and leaves after 6 s; a newer one restarts the clock (MapScreen.swift). */
const TOAST_MS = 6000;
/** An arrival the map never drew (a reload left it in a cluster) is forgotten after this, so the set can't grow. */
const FORGET_MS = 5000;
/** An arrival the map has started to animate: kept until it says it is done. */
const PLAYING = Number.POSITIVE_INFINITY;

interface Arrivals {
  /** Live arrivals in view the map hasn't animated yet, or is animating. */
  arrivals: ReadonlySet<string>;
  /** The newest severe or extreme arrival in view, until dismissed or 6 s pass. */
  toast: HazardEvent | undefined;
  /** A `created` change while live, in view and matching; `animate` false when the map already had it. */
  note: (event: HazardEvent, animate: boolean) => void;
  /** The map has begun animating the arrival: it is no longer forgotten. */
  started: (id: string) => void;
  /** The map has animated the arrival in. */
  arrived: (id: string) => void;
  dismissToast: () => void;
  /** Pointer or focus on the toast: its clock stops, and starts over when let go. */
  holdToast: (held: boolean) => void;
}

/** The arrivals not yet started that have waited past `FORGET_MS` at `now`, dropped. */
function pruned(pending: ReadonlyMap<string, number>, now: number): ReadonlyMap<string, number> {
  const kept = [...pending].filter(([, at]) => now - at <= FORGET_MS);
  return kept.length === pending.size ? pending : new Map(kept);
}

function without(pending: ReadonlyMap<string, number>, id: string): ReadonlyMap<string, number> {
  if (!pending.has(id)) {
    return pending;
  }
  return new Map([...pending].filter(([each]) => each !== id));
}

/** Forgets each arrival the map hasn't started, `FORGET_MS` after it came, whether or not another follows. */
function useForgetting(
  pending: ReadonlyMap<string, number>,
  setPending: (update: (current: ReadonlyMap<string, number>) => ReadonlyMap<string, number>) => void,
): void {
  useEffect(() => {
    const waiting = [...pending.values()].filter((at) => at !== PLAYING);
    if (waiting.length === 0) {
      return undefined;
    }
    const due = Math.min(...waiting) + FORGET_MS + 1 - Date.now();
    const timer = setTimeout(
      () => {
        setPending((current) => pruned(current, Date.now()));
      },
      Math.max(due, 0),
    );
    return () => {
      clearTimeout(timer);
    };
  }, [pending, setPending]);
}

/** The toast: the newest severe arrival, gone after 6 s unless held, or when dismissed. */
function useToast(): Pick<Arrivals, "toast" | "dismissToast" | "holdToast"> & { show: (event: HazardEvent) => void } {
  const [toast, setToast] = useState<HazardEvent>();
  const [held, setHeld] = useState(false);
  const dismissToast = useCallback(() => {
    setToast(undefined);
    setHeld(false);
  }, []);
  useEffect(() => {
    if (toast === undefined || held) {
      return undefined;
    }
    const timer = setTimeout(dismissToast, TOAST_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [toast, held, dismissToast]);
  return { toast, dismissToast, holdToast: setHeld, show: setToast };
}

/** New events as they arrive live (LiveMapModel arrivals): the map fades them in, a severe one is toasted. */
function useArrivals(): Arrivals {
  const [pending, setPending] = useState<ReadonlyMap<string, number>>(new Map());
  const { toast, dismissToast, holdToast, show } = useToast();
  useForgetting(pending, setPending);
  const note = useCallback(
    (event: HazardEvent, animate: boolean) => {
      if (animate) {
        const now = Date.now();
        setPending((current) => new Map([...pruned(current, now), [event.id, now]]));
      }
      if (isAtLeast(event.severity, "severe")) {
        show(event);
      }
    },
    [show],
  );
  const started = useCallback((id: string) => {
    setPending((current) => (current.has(id) ? new Map([...current, [id, PLAYING]]) : current));
  }, []);
  const arrived = useCallback((id: string) => {
    setPending((current) => without(current, id));
  }, []);
  const arrivals = useMemo(() => new Set(pending.keys()), [pending]);
  return { arrivals, toast, note, started, arrived, dismissToast, holdToast };
}

export type { Arrivals };
export { useArrivals };

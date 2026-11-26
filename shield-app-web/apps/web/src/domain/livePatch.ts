/**
 * What a live change does to the map's events (LiveMapModel `apply`). Patched in place only when the
 * response is the complete, unclustered set of events in view: zoom 6 or more, and not truncated.
 * Otherwise a change in view reloads, because only the server knows:
 * - below zoom 6, whether the event joins a cluster (even where none exists yet);
 * - when truncated, whether the event makes the cut.
 */
import type { EventList, HazardEvent } from "@shield/api-client";

import type { EventQuery } from "./filters.ts";
import { matches } from "./filters.ts";
import { contains, geoBox } from "./geo.ts";
import type { View } from "./viewport.ts";

/** Where the server stops clustering (ADR 9). */
const DETAIL_ZOOM = 6;
/**
 * More patches than this within a second is a burst: a source's first load (Meteoalarm sends
 * ~3,900 at once) or a backlog after a reconnect. Past it the changes fold into one reload, which
 * the server answers with the same state in a single response.
 */
const BURST_LIMIT = 50;
const BURST_WINDOW_MS = 1000;

interface PatchContext {
  view: View;
  query: EventQuery;
  now: Date;
  /** Counts a patch about to happen; true once this second has seen more than the limit. */
  inBurst: () => boolean;
}

type Outcome = { kind: "none" } | { kind: "reload" } | { kind: "patched"; list: EventList };

const NONE: Outcome = { kind: "none" };
const RELOAD: Outcome = { kind: "reload" };

function canPatch(list: EventList, view: View): boolean {
  return view.zoom >= DETAIL_ZOOM && !list.truncated;
}

function inView(view: View, event: HazardEvent): boolean {
  const box = geoBox(view.bbox.split(",").map(Number));
  return box === undefined || contains(box, event.centroid);
}

/** A new or changed event: in place, out (it stopped matching, or moved away), or a reload. */
function upsert(list: EventList, event: HazardEvent, context: PatchContext): Outcome {
  const shown = list.events.some((each) => each.id === event.id);
  const here = inView(context.view, event);
  if (!shown && !here) {
    return NONE;
  }
  if (!canPatch(list, context.view) || context.inBurst()) {
    return RELOAD;
  }
  const others = list.events.filter((each) => each.id !== event.id);
  const keep = here && matches(context.query, event, context.now);
  return { kind: "patched", list: { ...list, events: keep ? [...others, event] : others } };
}

/** A removed event: out of the list, or a reload when it may be a cluster member or lets a cut event in. */
function removal(list: EventList, id: string, context: PatchContext): Outcome {
  const shown = list.events.some((each) => each.id === id);
  if (canPatch(list, context.view) && !(shown && context.inBurst())) {
    return shown ? { kind: "patched", list: { ...list, events: list.events.filter((each) => each.id !== id) } } : NONE;
  }
  return shown || list.clusters.length > 0 || list.truncated ? RELOAD : NONE;
}

/** What `change` does to `list`, the response for the view and filters in `context`. */
function patchEvents(
  list: EventList,
  change: { kind: "upsert"; event: HazardEvent } | { kind: "removed"; id: string },
  context: PatchContext,
): Outcome {
  return change.kind === "upsert" ? upsert(list, change.event, context) : removal(list, change.id, context);
}

/** A meter for `PatchContext.inBurst`: true past `BURST_LIMIT` patches within a second. */
function burstMeter(now: () => number = Date.now): () => boolean {
  let start = Number.NEGATIVE_INFINITY;
  let count = 0;
  return () => {
    const at = now();
    if (at - start > BURST_WINDOW_MS) {
      start = at;
      count = 0;
    }
    count += 1;
    return count > BURST_LIMIT;
  };
}

/** Whether `event` is news for this view and filters: in view and matching (LiveMapModel `noteArrival`). */
function isNews(event: HazardEvent, context: Pick<PatchContext, "view" | "query" | "now">): boolean {
  return inView(context.view, event) && matches(context.query, event, context.now);
}

export type { Outcome, PatchContext };
export { burstMeter, isNews, patchEvents };

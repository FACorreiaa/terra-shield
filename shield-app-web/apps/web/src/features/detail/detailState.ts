import type { components, HazardEvent } from "@shield/api-client";

type EventDetail = components["schemas"]["EventDetail"];

/** What `GET /v1/events/{id}` answered: the event, or that its source no longer publishes it (404). */
type DetailAnswer = { found: true; detail: EventDetail } | { found: false };

/**
 * What the event detail (DESIGN.md §10.3) shows for the selection (LiveMapModel `DetailState`).
 * Gone carries the event as last seen, when it was, so the detail can still say what it was.
 */
type DetailState =
  | { kind: "loading" }
  | { kind: "loaded"; detail: EventDetail }
  | { kind: "gone"; last: HazardEvent | undefined }
  | { kind: "failed" };

/**
 * The state from the query: a failed refresh keeps the detail already shown (the query keeps its
 * data), only a first load that failed reads as failed.
 */
function detailState(answer: DetailAnswer | undefined, failed: boolean, last: HazardEvent | undefined): DetailState {
  if (answer?.found === true) {
    return { kind: "loaded", detail: answer.detail };
  }
  if (answer?.found === false) {
    return { kind: "gone", last };
  }
  return failed ? { kind: "failed" } : { kind: "loading" };
}

export type { DetailAnswer, DetailState, EventDetail };
export { detailState };

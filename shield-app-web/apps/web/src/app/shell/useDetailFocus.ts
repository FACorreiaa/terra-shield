import { useEffect, useRef } from "react";

import { useShowOnArrival } from "./useShowOnMap.ts";

/** The detail's title: focusable from script, so focus has somewhere to land when the list goes. */
const DETAIL_HEADING = "data-detail-heading";

/** Focus went with the element that held it (a row the sheet replaced, a Close button removed). */
function focusLost(): boolean {
  const active = document.activeElement;
  // A section hidden under the detail keeps its focused row mounted; focus there is lost to sight.
  return active === null || active === document.body || active.closest("[hidden]") !== null;
}

/** Puts keyboard focus on the list's row for `id`, once the list is in place. */
function focusRow(id: string): void {
  requestAnimationFrame(() => {
    document.querySelector<HTMLElement>(`[data-event-id="${CSS.escape(id)}"] button`)?.focus();
  });
}

/**
 * Opening an event where the pressed row disappears (the phone's sheet shows the detail in place
 * of the list) moves focus to the detail's title, not to the top of the page.
 */
function useFocusOnOpen(selectedId: string | undefined): void {
  useEffect(() => {
    if (selectedId !== undefined && focusLost()) {
      document.querySelector<HTMLElement>(`[${DETAIL_HEADING}]`)?.focus();
    }
  }, [selectedId]);
}

/** Closing the detail (Close, Escape, Back) returns focus to the event's row when it was lost. */
function useFocusReturn(selectedId: string | undefined): void {
  const previous = useRef(selectedId);
  useEffect(() => {
    const closed = previous.current;
    previous.current = selectedId;
    if (selectedId === undefined && closed !== undefined && focusLost()) {
      focusRow(closed);
    }
  }, [selectedId]);
}

/**
 * Closing an aircraft's report where focus went with it returns focus to the flights switch: the
 * aircraft itself is drawn on the canvas, with nothing to focus.
 */
function useAircraftFocusReturn(aircraftHex: string | undefined): void {
  const previous = useRef(aircraftHex);
  useEffect(() => {
    const closed = previous.current;
    previous.current = aircraftHex;
    if (aircraftHex === undefined && closed !== undefined && focusLost()) {
      requestAnimationFrame(() => {
        document.querySelector<HTMLElement>("button[aria-label='Live flights']")?.focus();
      });
    }
  }, [aircraftHex]);
}

/** What the frame does as the selection comes and goes: frames an arrival, gives focus back on close. */
function useSelectionUpkeep(selectedId: string | undefined, aircraftHex: string | undefined): void {
  useShowOnArrival();
  useFocusReturn(selectedId);
  useAircraftFocusReturn(aircraftHex);
}

export { DETAIL_HEADING, focusLost, focusRow, useFocusOnOpen, useSelectionUpkeep };

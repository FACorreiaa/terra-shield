/**
 * Search on the live map (DESIGN.md §10 shells; MapSearch.swift). No search endpoint exists, so it
 * covers what the app can answer itself: countries from `/v1/countries`, and events in view.
 */
import type { HazardEvent } from "@shield/api-client";

import { fold } from "./text.ts";

/** What the results say, verbatim from the apps where they have it. */
const SEARCH_WORDS = {
  placeholder: "Search countries or events",
  hint: "Type a country, or part of an event’s title.",
  noEvent: "No event in view matches.",
  clustered: "Events grouped on the map aren’t searched. Zoom in to include them.",
} as const;

/** Events whose title holds the text, ignoring case and accents, in the order given (the map's). */
function eventsMatching<Event extends Pick<HazardEvent, "title">>(text: string, events: readonly Event[]): Event[] {
  const query = fold(text.trim());
  if (query === "") {
    return [];
  }
  return events.filter((event) => fold(event.title).includes(query));
}

/** "No active events", "1 active event", "12 active events". */
function activeEventWords(count: number): string {
  if (count === 0) {
    return "No active events";
  }
  return count === 1 ? "1 active event" : `${count} active events`;
}

/** What a screen reader hears as results change: "No results", "1 result", "5 results". */
function resultWords(count: number): string {
  if (count === 0) {
    return "No results";
  }
  return count === 1 ? "1 result" : `${count} results`;
}

export { activeEventWords, eventsMatching, resultWords, SEARCH_WORDS };

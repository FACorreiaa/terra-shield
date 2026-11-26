import { use, useEffect, useRef } from "react";

import type { Country } from "@/domain/countries.ts";
import { mapBox } from "@/domain/countries.ts";
import { COUNTRY_HEADING } from "@/features/countries/CountryHeader.tsx";
import type { CountryState } from "@/features/countries/countryState.ts";
import { fitBox } from "@/features/map/fitBox.ts";
import { whenChromeRests } from "@/features/map/mapInsets.ts";

import { useLiveMap } from "./liveMapContext.ts";
import { SheetDetentSetter } from "./sheetDetents.ts";

/** A country is framed with this room around it, and never closer than a region's zoom. */
const COUNTRY_FIT = { breathingRoom: 32, maxZoom: 8 } as const;

/**
 * The box to frame once the overview has answered: the summary's, whose events place a country
 * across ±180° (Fiji, Russia); without one, the list's boundary; undefined while it loads.
 */
function arrivalBox(state: CountryState, known: Country | undefined): ReturnType<typeof mapBox> | undefined {
  if (state.kind === "loading") {
    return undefined;
  }
  if (state.kind === "loaded") {
    return mapBox(state.summary.country, state.summary.events);
  }
  return known === undefined ? undefined : mapBox(known, []);
}

/**
 * Opening a country (DESIGN.md §10.2) frames it on the map once per arrival (`arrival` names one:
 * the code, and how often it was chosen again), after the answer and once the sheet or inspector
 * has settled; never again on the minute's refresh or when an event opened from it closes. A link
 * that also opens an event leaves the camera to the event.
 */
function useCountryArrival(arrival: string, state: CountryState, known: Country | undefined): void {
  const { map, selectedId } = useLiveMap();
  const framed = useRef<string>(undefined);
  const pending = useRef<() => void>(undefined);

  const box = arrivalBox(state, known);
  useEffect(() => {
    if (framed.current === arrival || box === undefined || map === undefined) {
      return;
    }
    framed.current = arrival;
    if (selectedId !== undefined) {
      return;
    }
    pending.current = whenChromeRests(() => {
      fitBox(map, box, COUNTRY_FIT);
    });
  }, [arrival, box, map, selectedId]);
  // Leaving the country before the chrome settles drops its camera move.
  useEffect(() => {
    const cancel = pending;
    return () => {
      cancel.current?.();
    };
  }, []);
}

/**
 * Rendered keyed by the country's code, so each country opened lifts the phone's sheet to half
 * height (the country stays in view above it, as in the apps) and puts keyboard focus on its name.
 */
function CountryOpened(): undefined {
  const setDetent = use(SheetDetentSetter);
  useEffect(() => {
    setDetent?.("medium");
    requestAnimationFrame(() => {
      document.querySelector<HTMLElement>(`[${COUNTRY_HEADING}]`)?.focus();
    });
  }, [setDetent]);
  return undefined;
}

export { arrivalBox, CountryOpened, useCountryArrival };

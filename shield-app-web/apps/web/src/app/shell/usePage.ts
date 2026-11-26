import { useMatch, useNavigate } from "@tanstack/react-router";
import { useCallback, useSyncExternalStore } from "react";

interface Page {
  /** The Monitor's map and list. */
  onMap: boolean;
  /** The Countries list. */
  listing: boolean;
  /** The open country's code as written in the address, on its overview. */
  country: string | undefined;
}

/** Which section the address shows, by route rather than by path text (a trailing slash, a case). */
function usePage(): Page {
  const onMap = useMatch({ from: "/_app/map", shouldThrow: false }) !== undefined;
  const listing = useMatch({ from: "/_app/countries/", shouldThrow: false }) !== undefined;
  const country = useMatch({
    from: "/_app/countries/$iso2",
    shouldThrow: false,
    select: (match) => match.params.iso2,
  });
  return { onMap, listing, country };
}

/** Keyboard focus after a country closes: its row in the list, else the list's filter. */
function focusCountryRow(code: string | undefined): void {
  const row = code === undefined ? undefined : document.querySelector<HTMLElement>(`[data-country-row="${code}"]`);
  (row ?? document.querySelector<HTMLElement>("[data-country-filter]"))?.focus();
}

/** Closes a country's overview up to the Countries list, and puts keyboard focus on its row there. */
function useCloseCountry(): (code: string | undefined) => void {
  const navigate = useNavigate();
  return useCallback(
    (code: string | undefined) => {
      const close = async (): Promise<void> => {
        await navigate({ to: "/countries" });
        requestAnimationFrame(() => {
          focusCountryRow(code);
        });
      };
      void close();
    },
    [navigate],
  );
}

// Choosing the country already open (from search) opens it afresh: framed again, focus on its name.
// The address doesn't change, so the open country counts these instead.
let reopens = 0;
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function reopenCountry(): void {
  reopens += 1;
  for (const listener of listeners) {
    listener();
  }
}

function currentReopens(): number {
  return reopens;
}

/** How many times the open country was chosen again; part of what makes an arrival new. */
function useReopens(): number {
  return useSyncExternalStore(subscribe, currentReopens);
}

export { reopenCountry, useCloseCountry, usePage, useReopens };

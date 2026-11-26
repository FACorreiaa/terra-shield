import { useNavigate } from "@tanstack/react-router";
import type { RefObject } from "react";
import { useEffect, useEffectEvent } from "react";

import { officialUrl, sourceOf } from "@/features/detail/officialSource.ts";
import { mapPrefs } from "@/features/map/mapPrefs.ts";
import { fitWorld } from "@/features/map/useHazardMap.ts";

import type { LiveMap } from "./liveMapContext.ts";
import { useLiveMap } from "./liveMapContext.ts";
import type { MapSearch } from "./mapSearchContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import type { Section } from "./sections.ts";
import { SECTIONS } from "./sections.ts";
import type { Shortcut } from "./shortcuts.ts";
import { neighbour, shortcutFor } from "./shortcuts.ts";
import { useCloseCountry, usePage } from "./usePage.ts";

interface ShortcutTargets {
  toggleInspector: () => void;
  help: RefObject<HTMLDialogElement | null>;
}

interface Context extends ShortcutTargets {
  live: LiveMap;
  search: MapSearch;
  goTo: (to: Section["to"]) => void;
  /** ↑/↓ walk the map's list only where it shows. */
  onMap: boolean;
  /** Closes an open country's overview, up to the Countries list; false with none open. */
  closeCountry: () => boolean;
}

/** Each action, run with the app's current state; true when it did something (the key is then consumed). */
type Handler = (context: Context, shortcut: Shortcut) => boolean;

/** Brings a newly selected list row into view, so ↑/↓ never select something off screen. */
function revealSelected(id: string): void {
  requestAnimationFrame(() => {
    document.querySelector(`[data-event-id="${CSS.escape(id)}"]`)?.scrollIntoView({ block: "nearest" });
  });
}

function step({ live, search, onMap }: Context, direction: 1 | -1): boolean {
  // While searching, the list under the results is hidden, and other sections don't show it:
  // stepping through it would select blind.
  if (search.active || !onMap) {
    return false;
  }
  const next = neighbour(live.list, live.selectedId, direction);
  if (next === undefined) {
    return false;
  }
  live.select(next.id);
  revealSelected(next.id);
  return true;
}

const HANDLERS: Readonly<Record<Shortcut["action"], Handler>> = {
  section: ({ goTo }, shortcut) => {
    const section = "key" in shortcut ? SECTIONS.find((each) => each.key === shortcut.key) : undefined;
    if (section !== undefined) {
      goTo(section.to);
    }
    return section !== undefined;
  },
  next: (context) => step(context, 1),
  previous: (context) => step(context, -1),
  // Search, then the event, then the country (MapScreen.swift).
  clear: ({ live, search, closeCountry }) => {
    if (search.active) {
      search.end();
      return true;
    }
    if (live.selectedId !== undefined) {
      live.select(undefined);
      return true;
    }
    return closeCountry();
  },
  source: ({ live: { selected, sources, detail } }) => {
    const source = selected === undefined ? undefined : sourceOf(selected, sources, detail);
    const url = selected === undefined ? undefined : officialUrl(selected, source, detail?.kind === "gone");
    if (url !== undefined) {
      globalThis.open(url, "_blank", "noopener,noreferrer");
    }
    return url !== undefined;
  },
  inspector: ({ toggleInspector }) => {
    toggleInspector();
    return true;
  },
  world: ({ live: { map } }) => {
    if (map !== undefined) {
      fitWorld(map, mapPrefs().projection);
    }
    return map !== undefined;
  },
  refresh: ({ live }) => {
    void live.events.refetch();
    return true;
  },
  search: ({ search }) => search.focus(),
  help: ({ help }) => {
    help.current?.showModal();
    return true;
  },
};

/** Binds the shortcuts in `shortcuts.ts` to the app: sections, selection, map and refresh. */
function useShortcuts(targets: ShortcutTargets): void {
  const live = useLiveMap();
  const search = useMapSearch();
  const navigate = useNavigate();
  const { onMap, country } = usePage();
  const close = useCloseCountry();
  const run = useEffectEvent((shortcut: Shortcut): boolean => {
    const closeCountry = (): boolean => {
      if (country === undefined) {
        return false;
      }
      close(country.toUpperCase());
      return true;
    };
    const goTo = (to: Section["to"]): void => {
      // As the sidebar's links: the selected event stays open beside the section.
      void navigate({ to, search: (previous) => ({ event: previous.event }) });
    };
    return HANDLERS[shortcut.action]({ ...targets, live, search, goTo, onMap, closeCountry }, shortcut);
  });

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      const shortcut = shortcutFor(event);
      if (shortcut !== undefined && run(shortcut)) {
        event.preventDefault();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
    };
  }, []);
}

export { useShortcuts };

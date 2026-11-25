import { useNavigate } from "@tanstack/react-router";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { RefObject } from "react";
import { useEffect, useEffectEvent } from "react";

import { mapPrefs } from "@/features/map/mapPrefs.ts";
import { fitWorld } from "@/features/map/useHazardMap.ts";

import type { LiveMap } from "./liveMapContext.ts";
import { useLiveMap } from "./liveMapContext.ts";
import { SECTIONS } from "./sections.ts";
import type { Shortcut } from "./shortcuts.ts";
import { neighbour, shortcutFor } from "./shortcuts.ts";

interface ShortcutTargets {
  map: MapLibreMap | undefined;
  toggleInspector: () => void;
  help: RefObject<HTMLDialogElement | null>;
}

interface Context extends ShortcutTargets {
  live: LiveMap;
  goTo: (to: "/map") => void;
}

/** Each action, run with the app's current state; true when it did something (the key is then consumed). */
type Handler = (context: Context, shortcut: Shortcut) => boolean;

/** Brings a newly selected list row into view, so ↑/↓ never select something off screen. */
function revealSelected(id: string): void {
  requestAnimationFrame(() => {
    document.querySelector(`[data-event-id="${CSS.escape(id)}"]`)?.scrollIntoView({ block: "nearest" });
  });
}

function step({ live }: Context, direction: 1 | -1): boolean {
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
  clear: ({ live }) => {
    live.select(undefined);
    return live.selectedId !== undefined;
  },
  source: ({ live }) => {
    const url = live.selected?.url;
    if (url !== undefined) {
      globalThis.open(url, "_blank", "noopener,noreferrer");
    }
    return url !== undefined;
  },
  inspector: ({ toggleInspector }) => {
    toggleInspector();
    return true;
  },
  world: ({ map }) => {
    if (map !== undefined) {
      fitWorld(map, mapPrefs().projection);
    }
    return map !== undefined;
  },
  refresh: ({ live }) => {
    void live.events.refetch();
    return true;
  },
  help: ({ help }) => {
    help.current?.showModal();
    return true;
  },
};

/** Binds the shortcuts in `shortcuts.ts` to the app: sections, selection, map and refresh. */
function useShortcuts(targets: ShortcutTargets): void {
  const live = useLiveMap();
  const navigate = useNavigate();
  const run = useEffectEvent((shortcut: Shortcut): boolean => {
    const goTo = (to: "/map"): void => {
      void navigate({ to });
    };
    return HANDLERS[shortcut.action]({ ...targets, live, goTo }, shortcut);
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

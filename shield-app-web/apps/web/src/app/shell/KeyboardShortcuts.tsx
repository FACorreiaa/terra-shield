import type { ReactElement } from "react";
import { useRef } from "react";

import { useFlightsLayer } from "@/features/flights/flightsContext.ts";

import { ShortcutsHelp } from "./ShortcutsHelp.tsx";
import { useShortcuts } from "./useShortcuts.ts";

interface KeyboardShortcutsProps {
  toggleInspector: () => void;
}

/** The app's keyboard shortcuts and the `?` sheet that lists them. */
function KeyboardShortcuts({ toggleInspector }: KeyboardShortcutsProps): ReactElement {
  const help = useRef<HTMLDialogElement>(null);
  const flights = useFlightsLayer();
  useShortcuts({ toggleInspector, help, flights });
  return <ShortcutsHelp dialog={help} />;
}

export { KeyboardShortcuts };

import type { ReactElement } from "react";
import { useRef } from "react";

import { ShortcutsHelp } from "./ShortcutsHelp.tsx";
import { useShortcuts } from "./useShortcuts.ts";

interface ShortcutsProps {
  toggleInspector: () => void;
}

/** The app's keyboard shortcuts and the `?` sheet that lists them. */
function Shortcuts({ toggleInspector }: ShortcutsProps): ReactElement {
  const help = useRef<HTMLDialogElement>(null);
  useShortcuts({ toggleInspector, help });
  return <ShortcutsHelp dialog={help} />;
}

export { Shortcuts };

/**
 * The web's keyboard shortcuts (DESIGN.md §10 shells). The Mac's ⌘1–5, ⌘L, ⌘R and ⌘F belong to the
 * browser, so the web uses single keys instead, active only outside text fields (VIEWS.md, web
 * deviations); `/` stands in for ⌘F.
 */
type Shortcut =
  | { action: "section"; key: string }
  | {
      action:
        | "inspector"
        | "source"
        | "refresh"
        | "world"
        | "detections"
        | "conflicts"
        | "flights"
        | "next"
        | "previous"
        | "clear"
        | "help"
        | "search";
    };

interface KeyPress {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  defaultPrevented: boolean;
  /** Held down: a layer switch would flap with every repeat. */
  repeat?: boolean;
  target: EventTarget | null;
}

/** Shown in the help sheet, in this order. */
const SHORTCUT_HELP: readonly { keys: string; does: string }[] = [
  { keys: "1", does: "Monitor" },
  { keys: "2", does: "Countries" },
  { keys: "5", does: "Sources" },
  { keys: "/", does: "Search countries and events in view" },
  { keys: "↑ ↓", does: "Previous or next event in the list" },
  { keys: "Esc", does: "End the search, then clear the selection, then close the country" },
  { keys: "o", does: "Open the selected event's official source" },
  { keys: "i", does: "Show or hide the inspector" },
  { keys: "0", does: "Show the whole world" },
  { keys: "h", does: "Show or hide satellite heat detections" },
  { keys: "a", does: "Show or hide live flights" },
  { keys: "c", does: "Show or hide armed conflicts" },
  { keys: "r", does: "Refresh now" },
  { keys: "?", does: "These shortcuts" },
];

const SINGLE = new Map<string, Shortcut>([
  ["i", { action: "inspector" }],
  ["o", { action: "source" }],
  ["r", { action: "refresh" }],
  ["0", { action: "world" }],
  ["h", { action: "detections" }],
  ["a", { action: "flights" }],
  ["c", { action: "conflicts" }],
  ["ArrowDown", { action: "next" }],
  ["ArrowUp", { action: "previous" }],
  ["Escape", { action: "clear" }],
  ["?", { action: "help" }],
  ["/", { action: "search" }],
]);
const SECTION_KEYS = new Set(["1", "2", "3", "4", "5"]);
const LAYER_ACTIONS: ReadonlySet<string> = new Set(["detections", "conflicts", "flights"]);
/**
 * Typing here, or keys these already use: radios move with arrows, and a dialog closes on Escape
 * without also clearing the selection behind it.
 */
const OWNS_KEYS = "input, textarea, select, [contenteditable=''], [contenteditable='true'], [role='slider'], dialog";
/**
 * The map's own keys (MapLibre's keyboard handler): it pans with arrows and zooms with + − =. Every
 * other key still works with the map focused, so Escape closes what a click on it opened.
 */
const MAP_KEYS: ReadonlySet<string> = new Set(["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "+", "-", "=", "_"]);

function ownedBy(target: EventTarget | null, key: string): boolean {
  if (!(target instanceof Element)) {
    return false;
  }
  return target.closest(OWNS_KEYS) !== null || (MAP_KEYS.has(key) && target.closest(".maplibregl-canvas") !== null);
}

/** The shortcut a key press means, or undefined when the key belongs to something else. */
function shortcutFor(press: KeyPress): Shortcut | undefined {
  if (press.ctrlKey || press.metaKey || press.altKey || press.defaultPrevented) {
    return undefined;
  }
  if (ownedBy(press.target, press.key)) {
    return undefined;
  }
  if (SECTION_KEYS.has(press.key)) {
    return { action: "section", key: press.key };
  }
  const shortcut = SINGLE.get(press.key);
  // A held layer key would flip the layer on and off with each repeat.
  return press.repeat === true && LAYER_ACTIONS.has(shortcut?.action ?? "") ? undefined : shortcut;
}

/** The event after (or before) the selected one in display order; the first when nothing is selected. */
function neighbour<Item extends { id: string }>(
  list: readonly Item[],
  selectedId: string | undefined,
  step: 1 | -1,
): Item | undefined {
  const index = list.findIndex((item) => item.id === selectedId);
  if (index === -1) {
    return step === 1 ? list[0] : list.at(-1);
  }
  return list[Math.min(list.length - 1, Math.max(0, index + step))];
}

export type { KeyPress, Shortcut };
export { neighbour, SHORTCUT_HELP, shortcutFor };

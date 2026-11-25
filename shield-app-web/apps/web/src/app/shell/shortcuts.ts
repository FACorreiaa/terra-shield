/**
 * The web's keyboard shortcuts (DESIGN.md §10 shells). The Mac's ⌘1–5, ⌘L, ⌘R and ⌘F belong to the
 * browser, so the web uses single keys instead, active only outside text fields (VIEWS.md, web
 * deviations).
 */
type Shortcut =
  | { action: "section"; key: string }
  | { action: "inspector" | "source" | "refresh" | "world" | "next" | "previous" | "clear" | "help" };

interface KeyPress {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  defaultPrevented: boolean;
  target: EventTarget | null;
}

/** Shown in the help sheet, in this order. */
const SHORTCUT_HELP: readonly { keys: string; does: string }[] = [
  { keys: "1", does: "Monitor" },
  { keys: "↑ ↓", does: "Previous or next event in the list" },
  { keys: "Esc", does: "Clear the selection" },
  { keys: "o", does: "Open the selected event's official source" },
  { keys: "i", does: "Show or hide the inspector" },
  { keys: "0", does: "Show the whole world" },
  { keys: "r", does: "Refresh now" },
  { keys: "?", does: "These shortcuts" },
];

const SINGLE = new Map<string, Shortcut>([
  ["i", { action: "inspector" }],
  ["o", { action: "source" }],
  ["r", { action: "refresh" }],
  ["0", { action: "world" }],
  ["ArrowDown", { action: "next" }],
  ["ArrowUp", { action: "previous" }],
  ["Escape", { action: "clear" }],
  ["?", { action: "help" }],
]);
const SECTION_KEYS = new Set(["1", "2", "3", "4", "5"]);
/**
 * Typing here, or keys these already use: the map pans with arrows, radios move with them, and a
 * dialog closes on Escape without also clearing the selection behind it.
 */
const OWNS_KEYS =
  "input, textarea, select, [contenteditable=''], [contenteditable='true'], .maplibregl-canvas, [role='slider'], dialog";

/** The shortcut a key press means, or undefined when the key belongs to something else. */
function shortcutFor(press: KeyPress): Shortcut | undefined {
  if (press.ctrlKey || press.metaKey || press.altKey || press.defaultPrevented) {
    return undefined;
  }
  if (press.target instanceof Element && press.target.closest(OWNS_KEYS) !== null) {
    return undefined;
  }
  if (SECTION_KEYS.has(press.key)) {
    return { action: "section", key: press.key };
  }
  return SINGLE.get(press.key);
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

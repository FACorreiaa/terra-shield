import { describe, expect, it } from "vite-plus/test";

import type { KeyPress } from "./shortcuts.ts";
import { neighbour, shortcutFor } from "./shortcuts.ts";

function press(key: string, extra: Partial<KeyPress> = {}): KeyPress {
  return {
    key,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    defaultPrevented: false,
    target: document.body,
    ...extra,
  };
}

describe(shortcutFor, () => {
  it("toggles satellite heat detections with h and armed conflicts with c, never on a held key", () => {
    expect(shortcutFor(press("h"))).toStrictEqual({ action: "detections" });
    expect(shortcutFor({ ...press("h"), repeat: true })).toBeUndefined();
    expect(shortcutFor(press("c"))).toStrictEqual({ action: "conflicts" });
    expect(shortcutFor({ ...press("c"), repeat: true })).toBeUndefined();
  });

  it("maps the single keys", () => {
    expect(shortcutFor(press("i"))).toStrictEqual({ action: "inspector" });
    expect(shortcutFor(press("0"))).toStrictEqual({ action: "world" });
    expect(shortcutFor(press("Escape"))).toStrictEqual({ action: "clear" });
    expect(shortcutFor(press("1"))).toStrictEqual({ action: "section", key: "1" });
    expect(shortcutFor(press("x"))).toBeUndefined();
  });

  it("leaves the browser's own shortcuts alone", () => {
    expect(shortcutFor(press("r", { metaKey: true }))).toBeUndefined();
    expect(shortcutFor(press("1", { ctrlKey: true }))).toBeUndefined();
    expect(shortcutFor(press("0", { altKey: true }))).toBeUndefined();
  });

  it("never fires while typing or where the keys already mean something", () => {
    const input = document.createElement("input");
    const canvas = document.createElement("canvas");
    canvas.className = "maplibregl-canvas";
    expect(shortcutFor(press("r", { target: input }))).toBeUndefined();
    expect(shortcutFor(press("ArrowDown", { target: canvas }))).toBeUndefined();
    expect(shortcutFor(press("ArrowUp", { defaultPrevented: true }))).toBeUndefined();
    const dialog = document.createElement("dialog");
    dialog.append(document.createElement("button"));
    expect(shortcutFor(press("Escape", { target: dialog.firstElementChild }))).toBeUndefined();
  });
});

describe(neighbour, () => {
  const list = [{ id: "a" }, { id: "b" }, { id: "c" }];

  it("steps through the list and stops at its ends", () => {
    expect(neighbour(list, "a", 1)?.id).toBe("b");
    expect(neighbour(list, "c", 1)?.id).toBe("c");
    expect(neighbour(list, "a", -1)?.id).toBe("a");
  });

  it("starts at the top going down and at the bottom going up", () => {
    expect(neighbour(list, undefined, 1)?.id).toBe("a");
    expect(neighbour(list, undefined, -1)?.id).toBe("c");
    expect(neighbour<{ id: string }>([], undefined, 1)).toBeUndefined();
  });
});

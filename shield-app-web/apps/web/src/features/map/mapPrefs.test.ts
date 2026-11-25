import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { mapPrefs, resetMapPrefsForTests, setProjection, subscribeMapPrefs } from "./mapPrefs.ts";

const KEY = "shield.map";
/** The stored blob's one-letter version field (`lib/storage`). */
const VERSION_FIELD = "v";

/** How many times a mock ran (the lint rules disagree on asserting "once" directly). */
function calls(listener: { mock: { calls: unknown[] } }): number {
  return listener.mock.calls.length;
}

function stored(version: number, projection: string): string {
  return JSON.stringify({ [VERSION_FIELD]: version, data: { projection } });
}

describe("map prefs", () => {
  beforeEach(() => {
    localStorage.clear();
    resetMapPrefsForTests();
  });

  afterEach(() => {
    localStorage.clear();
  });

  describe(mapPrefs, () => {
    it("starts on the globe", () => {
      expect(mapPrefs().projection).toBe("globe");
    });

    it("reads a saved choice", () => {
      localStorage.setItem(KEY, stored(1, "mercator"));
      expect(mapPrefs().projection).toBe("mercator");
    });

    it("falls back to the globe for corrupt, old or unknown data", () => {
      for (const raw of ["{not json", stored(0, "mercator"), stored(1, "polar")]) {
        localStorage.setItem(KEY, raw);
        resetMapPrefsForTests();
        expect(mapPrefs().projection).toBe("globe");
      }
    });

    it("returns the same object until something changes", () => {
      expect(mapPrefs()).toBe(mapPrefs());
    });
  });

  describe(setProjection, () => {
    it("saves the choice and tells subscribers in this tab", () => {
      const listener = vi.fn<() => void>();
      const stop = subscribeMapPrefs(listener);
      setProjection("mercator");
      stop();
      expect(calls(listener)).toBe(1);
      expect(mapPrefs().projection).toBe("mercator");
      expect(localStorage.getItem(KEY)).toBe(stored(1, "mercator"));
    });

    it("does nothing when the choice is unchanged", () => {
      const listener = vi.fn<() => void>();
      const stop = subscribeMapPrefs(listener);
      setProjection("globe");
      stop();
      expect(listener).not.toHaveBeenCalled();
    });
  });

  describe(subscribeMapPrefs, () => {
    it("picks up a choice another tab saved", () => {
      const listener = vi.fn<() => void>();
      const stop = subscribeMapPrefs(listener);
      localStorage.setItem(KEY, stored(1, "mercator"));
      globalThis.dispatchEvent(new StorageEvent("storage", { key: KEY }));
      stop();
      expect(calls(listener)).toBe(1);
      expect(mapPrefs().projection).toBe("mercator");
    });
  });
});

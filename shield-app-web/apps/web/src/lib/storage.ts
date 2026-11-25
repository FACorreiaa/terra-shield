/**
 * Versioned values in `localStorage`, stored as `{ v, data }`. Storage can be missing, full, or
 * throw on access (a private window, blocked site data), so every access is guarded and anything
 * unreadable gives the fallback: a wrong version, corrupt JSON, or data the schema refuses.
 */
import type { ZodType } from "zod";

/** Stops a subscription. */
type Unsubscribe = () => void;

/** The stored object's version field, kept to one letter on disk. */
const VERSION_FIELD = "v";

function storage(): Storage | undefined {
  try {
    return globalThis.localStorage;
  } catch {
    return undefined;
  }
}

/** The value stored under `key` at `version`, or `fallback` when it can't be read. */
// oxlint-disable-next-line eslint/max-params -- the four parts of a stored value, each required
function readStored<Value>(key: string, version: number, schema: ZodType<Value>, fallback: Value): Value {
  try {
    const raw = storage()?.getItem(key);
    if (raw === null || raw === undefined) {
      return fallback;
    }
    const stored: unknown = JSON.parse(raw);
    if (typeof stored !== "object" || stored === null || !("v" in stored) || stored.v !== version) {
      return fallback;
    }
    const parsed = schema.safeParse("data" in stored ? stored.data : undefined);
    return parsed.success ? parsed.data : fallback;
  } catch {
    return fallback;
  }
}

/** Saves `value` under `key` at `version`; does nothing when storage refuses. */
function writeStored(key: string, version: number, value: unknown): void {
  try {
    storage()?.setItem(key, JSON.stringify({ [VERSION_FIELD]: version, data: value }));
  } catch {
    // Full or blocked: the value lives for this page only.
  }
}

/** Calls `onChange` when another tab writes or clears `key` (the `storage` event never fires in the writer). */
function subscribeStored(key: string, onChange: () => void): Unsubscribe {
  const listener = (event: StorageEvent): void => {
    // A null key is `localStorage.clear()`, which removes this key too.
    if (event.key === key || event.key === null) {
      onChange();
    }
  };
  try {
    globalThis.addEventListener("storage", listener);
  } catch {
    return () => {
      // Nothing was subscribed.
    };
  }
  return () => {
    globalThis.removeEventListener("storage", listener);
  };
}

export type { Unsubscribe };
export { readStored, subscribeStored, writeStored };

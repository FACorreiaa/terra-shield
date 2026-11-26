import { useSyncExternalStore } from "react";
import { z } from "zod";

import type { Unsubscribe } from "@/lib/storage.ts";
import { readStored, subscribeStored, writeStored } from "@/lib/storage.ts";

import type { Projection } from "./basemapStyle.ts";

/** How a person likes the map, kept across visits and shared between tabs (`shield.map` v1). */
interface MapPrefs {
  readonly projection: Projection;
}

const KEY = "shield.map";
const VERSION = 1;
const schema = z.object({ projection: z.enum(["globe", "mercator"]) });
/** The globe is the default (plan W2); the owner's call is only that both exist. */
const DEFAULTS: MapPrefs = { projection: "globe" };

const listeners = new Set<() => void>();
/** The last value read, so `useSyncExternalStore` sees the same object until it changes. */
const cache: { prefs: MapPrefs | undefined } = { prefs: undefined };

function mapPrefs(): MapPrefs {
  cache.prefs ??= readStored(KEY, VERSION, schema, DEFAULTS);
  return cache.prefs;
}

function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

function setProjection(projection: Projection): void {
  if (mapPrefs().projection === projection) {
    return;
  }
  cache.prefs = { ...mapPrefs(), projection };
  writeStored(KEY, VERSION, cache.prefs);
  notify();
}

/** Calls `listener` when the prefs change here or in another tab. */
function subscribeMapPrefs(listener: () => void): Unsubscribe {
  listeners.add(listener);
  const stopStorage = subscribeStored(KEY, () => {
    cache.prefs = undefined;
    listener();
  });
  return () => {
    listeners.delete(listener);
    stopStorage();
  };
}

function useMapPrefs(): MapPrefs {
  return useSyncExternalStore(subscribeMapPrefs, mapPrefs, () => DEFAULTS);
}

/** Forgets the cached value so a test reads storage afresh. */
function resetMapPrefsForTests(): void {
  cache.prefs = undefined;
}

export type { MapPrefs };
export { mapPrefs, resetMapPrefsForTests, setProjection, subscribeMapPrefs, useMapPrefs };

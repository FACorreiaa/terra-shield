import { useSyncExternalStore } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import type { SavedFilters } from "@/domain/filterStore.ts";
import { decodeFilters, DEFAULT_SAVED, encodeFilters } from "@/domain/filterStore.ts";
import type { Unsubscribe } from "@/lib/storage.ts";
import { readRawStored, subscribeStored, writeRawStored } from "@/lib/storage.ts";

/**
 * The filters a person chose (DESIGN.md §10.6, persisted locally), shared between tabs. Stored under
 * `shield.filters` as the apps' own versioned blob, so its `showsDetections` survives until the heat
 * layer (W7) reads it.
 */
const KEY = "shield.filters";

const listeners = new Set<() => void>();
/** The last value read, so `useSyncExternalStore` sees the same object until it changes. */
const cache: { saved: SavedFilters | undefined } = { saved: undefined };

function savedFilters(): SavedFilters {
  cache.saved ??= decodeFilters(readRawStored(KEY));
  return cache.saved;
}

function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

/** Applies `query` at once: the map reloads behind the sheet (FilterSheet.swift). */
function setQuery(query: EventQuery): void {
  cache.saved = { ...savedFilters(), query };
  writeRawStored(KEY, encodeFilters(cache.saved));
  notify();
}

/** Calls `listener` when the filters change here or in another tab. */
function subscribeFilters(listener: () => void): Unsubscribe {
  listeners.add(listener);
  const stopStorage = subscribeStored(KEY, () => {
    cache.saved = undefined;
    listener();
  });
  return () => {
    listeners.delete(listener);
    stopStorage();
  };
}

function useSavedFilters(): SavedFilters {
  return useSyncExternalStore(subscribeFilters, savedFilters, () => DEFAULT_SAVED);
}

/** Forgets the cached value so a test reads storage afresh. */
function resetFilterPrefsForTests(): void {
  cache.saved = undefined;
}

export { resetFilterPrefsForTests, savedFilters, setQuery, subscribeFilters, useSavedFilters };

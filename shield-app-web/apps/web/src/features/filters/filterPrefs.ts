import { useSyncExternalStore } from "react";

import type { EventQuery } from "@/domain/filters.ts";
import type { SavedFilters } from "@/domain/filterStore.ts";
import { decodeFilters, DEFAULT_SAVED, encodeFilters } from "@/domain/filterStore.ts";
import type { Unsubscribe } from "@/lib/storage.ts";
import { readRawStored, subscribeStored, writeRawStored } from "@/lib/storage.ts";

/**
 * The filters a person chose (DESIGN.md §10.6, persisted locally), shared between tabs. Stored under
 * `shield.filters` as the apps' own versioned blob, with the heat layer's switch beside the query
 * (FilterStore.swift).
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

function save(saved: SavedFilters): void {
  cache.saved = saved;
  writeRawStored(KEY, encodeFilters(saved));
  notify();
}

/** Applies `query` at once: the map reloads behind the sheet (FilterSheet.swift). */
function setQuery(query: EventQuery): void {
  save({ ...savedFilters(), query });
}

/** Shows or hides satellite heat detections: one switch for the map control, the sheet and `h`. */
function setShowsDetections(showsDetections: boolean): void {
  if (savedFilters().showsDetections !== showsDetections) {
    save({ ...savedFilters(), showsDetections });
  }
}

/** Flips the heat layer from what is stored now, so no caller acts on a stale value. */
function toggleShowsDetections(): void {
  setShowsDetections(!savedFilters().showsDetections);
}

/** Shows or hides armed conflicts (ADR 27): the map control and `c`. */
function setShowsConflicts(showsConflicts: boolean): void {
  if (savedFilters().showsConflicts !== showsConflicts) {
    save({ ...savedFilters(), showsConflicts });
  }
}

function toggleShowsConflicts(): void {
  setShowsConflicts(!savedFilters().showsConflicts);
}

/**
 * The sheet's Reset: default filters and the heat layer off (FilterSheet.swift). The conflicts layer
 * stays as it is: the sheet doesn't offer it.
 */
function resetFilters(query: EventQuery): void {
  save({ ...savedFilters(), query, showsDetections: false });
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

export {
  resetFilterPrefsForTests,
  resetFilters,
  savedFilters,
  setQuery,
  setShowsConflicts,
  setShowsDetections,
  subscribeFilters,
  toggleShowsConflicts,
  toggleShowsDetections,
  useSavedFilters,
};

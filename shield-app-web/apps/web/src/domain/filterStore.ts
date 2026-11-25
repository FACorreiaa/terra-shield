import { SEVERITY_ORDER } from "@shield/api-client";
import { z } from "zod";

import type { EventQuery } from "./filters.ts";
import { DEFAULT_QUERY, EVENT_KINDS, WINDOWS } from "./filters.ts";
import { HAZARD_TYPES } from "./taxonomy.ts";

/**
 * Saved filters (BACKLOG S3.3: "persisted locally"), as pure functions over the stored string so
 * the caller owns storage. Same blob as the apps' FilterStore.
 */

/** The filters a person chose, kept across visits (BACKLOG S3.3). */
interface SavedFilters {
  readonly query: EventQuery;
  readonly showsDetections: boolean;
}

/**
 * Version 1 of the stored blob, shared with the apps. It holds what is *excluded*, so a type or
 * kind added later shows by default. `window` is seconds, absent for any time.
 */
const blobSchema = z.object({
  version: z.literal(1),
  excludedTypes: z.array(z.string()),
  minimumSeverity: z.string(),
  excludedKinds: z.array(z.string()),
  window: z.number().nullish(),
  showsDetections: z.boolean(),
  // Added after version 1 shipped, so optional: a blob without it still reads.
  includeResolved: z.boolean().optional(),
});

const DEFAULT_SAVED: SavedFilters = { query: DEFAULT_QUERY, showsDetections: false };

/** The blob for `saved`, as a string for storage. */
function encodeFilters(saved: SavedFilters): string {
  const { query } = saved;
  const blob: z.input<typeof blobSchema> = {
    version: 1,
    excludedTypes: HAZARD_TYPES.filter((type) => !query.types.has(type)),
    minimumSeverity: query.minimumSeverity,
    excludedKinds: EVENT_KINDS.filter((kind) => !query.kinds.has(kind)),
    ...(query.windowSeconds === undefined ? {} : { window: query.windowSeconds }),
    showsDetections: saved.showsDetections,
    includeResolved: query.includeResolved,
  };
  return JSON.stringify(blob);
}

function parseBlob(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

/**
 * The saved filters, read leniently: unknown values are dropped, a window the sheet doesn't offer
 * falls back, a query that would show nothing is never restored, and anything unreadable gives the
 * defaults.
 */
function decodeFilters(raw: string | null | undefined): SavedFilters {
  const parsed = blobSchema.safeParse(raw === null || raw === undefined ? undefined : parseBlob(raw));
  if (!parsed.success) {
    return DEFAULT_SAVED;
  }
  const blob = parsed.data;
  const excludedTypes = new Set(blob.excludedTypes);
  const excludedKinds = new Set(blob.excludedKinds);
  const types = HAZARD_TYPES.filter((type) => !excludedTypes.has(type));
  const kinds = EVENT_KINDS.filter((kind) => !excludedKinds.has(kind));
  const windowSeconds = typeof blob.window === "number" && WINDOWS.includes(blob.window) ? blob.window : undefined;
  const query: EventQuery = {
    ...DEFAULT_QUERY,
    types: types.length === 0 ? DEFAULT_QUERY.types : new Set(types),
    minimumSeverity: SEVERITY_ORDER.find((level) => level === blob.minimumSeverity) ?? DEFAULT_QUERY.minimumSeverity,
    kinds: kinds.length === 0 ? DEFAULT_QUERY.kinds : new Set(kinds),
    windowSeconds: windowSeconds ?? DEFAULT_QUERY.windowSeconds,
    includeResolved: blob.includeResolved ?? DEFAULT_QUERY.includeResolved,
  };
  return { query, showsDetections: blob.showsDetections };
}

export type { SavedFilters };
export { decodeFilters, DEFAULT_SAVED, encodeFilters };

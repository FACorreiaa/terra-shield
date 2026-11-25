import type { Source } from "@shield/api-client";
import { useMemo } from "react";

import { isStale } from "@/domain/sources.ts";
import type { SourceLookup } from "@/features/events/sourceLookup.ts";
import { useSources } from "@/features/sources/useSources.ts";

interface SourceHealth {
  /** Undefined until the list has loaded; the detail then falls back on its own source record. */
  sources: readonly Source[] | undefined;
  /** Sources that have missed three polls (DECISIONS #8): their events fade, never hide. */
  staleSources: ReadonlySet<string>;
  /** The same, for list rows. */
  lookup: SourceLookup;
}

function useSourceHealth(now: Date): SourceHealth {
  const sources = useSources().data;
  const staleSources = useMemo(
    () => new Set((sources ?? []).filter((source) => isStale(source, now)).map((source) => source.id)),
    [sources, now],
  );
  const lookup = useMemo(
    () => ({ byId: new Map((sources ?? []).map((source) => [source.id, source])), stale: staleSources }),
    [sources, staleSources],
  );
  return { sources, staleSources, lookup };
}

// The rows read it from here, so the provider needs one module for both.
export { SourceLookupContext } from "@/features/events/sourceLookup.ts";
export { useSourceHealth };

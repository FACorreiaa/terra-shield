import type { Source } from "@shield/api-client";
import { useMemo } from "react";

import { isStale } from "@/domain/sources.ts";
import { useSources } from "@/features/sources/useSources.ts";

interface SourceHealth {
  /** Undefined until the list has loaded; the detail then falls back on its own source record. */
  sources: readonly Source[] | undefined;
  /** Sources that have missed three polls (DECISIONS #8): their events fade, never hide. */
  staleSources: ReadonlySet<string>;
}

function useSourceHealth(now: Date): SourceHealth {
  const sources = useSources().data;
  const staleSources = useMemo(
    () => new Set((sources ?? []).filter((source) => isStale(source, now)).map((source) => source.id)),
    [sources, now],
  );
  return { sources, staleSources };
}

export { useSourceHealth };

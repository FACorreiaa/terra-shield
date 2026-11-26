import { createFileRoute } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { useCallback, useEffect } from "react";

import { useLiveMap } from "@/app/shell/liveMapContext.ts";
import { useIsWide } from "@/app/shell/useBreakpoint.ts";
import { SourcesView } from "@/features/sources/SourcesView.tsx";
import { useSources } from "@/features/sources/useSources.ts";
import { useRefreshOnOpen } from "@/lib/useRefreshOnOpen.ts";

/** Sources & attribution (`/sources`, key 5), refreshed each time it opens. */
function SourcesSection(): ReactElement {
  const { data, isError, refetch } = useSources();
  useRefreshOnOpen(refetch);
  const { now, selectedId, select } = useLiveMap();
  const wide = useIsWide();
  // On a phone the sheet shows an open event over the section: Sources closes it (key 5 keeps it
  // elsewhere), replacing the entry, so Back and Done return to it.
  useEffect(() => {
    if (!wide && selectedId !== undefined) {
      select(undefined);
    }
  }, [wide, selectedId, select]);
  const retry = useCallback(() => {
    void refetch();
  }, [refetch]);
  return (
    <section aria-label="Sources">
      {/* On a phone the title, with Done, sits in the sheet's pinned header. */}
      {wide ? <h1 className="px-4 pt-3 text-title text-ink">Sources</h1> : undefined}
      <SourcesView sources={data} failed={isError} onRetry={retry} now={now} />
    </section>
  );
}

export const Route = createFileRoute("/_app/sources")({ component: SourcesSection });

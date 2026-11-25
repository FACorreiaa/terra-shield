import { createFileRoute } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { useLiveMap } from "@/app/shell/liveMapContext.ts";
import { MonitorBody } from "@/app/shell/MonitorBody.tsx";
import { useIsWide } from "@/app/shell/useBreakpoint.ts";

/**
 * Monitor (DESIGN.md §10.1): the events in view, most severe first. It is the sidebar's body on a
 * wide screen and the sheet's on a phone, where the sheet's header already names it.
 */
function MonitorSection(): ReactElement {
  const { shown, list } = useLiveMap();
  const wide = useIsWide();
  const count = shown === undefined ? "–" : `${list.length}${shown.truncated ? "+" : ""}`;
  return (
    <section aria-label="Live events">
      {wide ? (
        <div className="flex items-baseline justify-between px-4 py-3">
          <h1 className="text-title text-ink">Live events</h1>
          <span className="font-mono text-meta text-muted">{count}</span>
        </div>
      ) : undefined}
      <MonitorBody />
    </section>
  );
}

export const Route = createFileRoute("/_app/map")({ component: MonitorSection });

import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { overallHealth } from "@/domain/sources.ts";
import { HealthDot } from "@/features/sources/HealthDot.tsx";
import { keepSelection } from "@/lib/keepSelection.ts";

import { useLiveMap } from "./liveMapContext.ts";
import { SOURCES_SECTION } from "./sections.ts";

/**
 * Sources, pinned at the foot of the sidebar with the sources' overall health (Sidebar.swift
 * `SourcesSidebarRow`). No dot before the first list arrives: no health is not good health.
 */
function SourcesNavRow(): ReactElement {
  const { sources, now } = useLiveMap();
  const health = overallHealth(sources, now);
  const { icon: Icon, label, key } = SOURCES_SECTION;
  return (
    <nav aria-label="Sources" className="border-t border-hairline px-2 py-2">
      <Link
        to="/sources"
        search={keepSelection}
        title={`${label} (${key})`}
        className="flex min-h-11 items-center gap-3 rounded-sm px-3 text-body text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent aria-[current=page]:bg-raised aria-[current=page]:text-ink"
      >
        <Icon className="size-4" aria-hidden />
        <span className="flex-1">{label}</span>
        {health === undefined ? undefined : <HealthDot health={health} />}
      </Link>
    </nav>
  );
}

export { SourcesNavRow };

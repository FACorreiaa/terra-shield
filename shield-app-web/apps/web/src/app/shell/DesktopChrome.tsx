import { Link } from "@tanstack/react-router";
import type { ReactElement, ReactNode } from "react";
import { useCallback, useLayoutEffect } from "react";

import { EventSummary } from "@/features/events/EventSummary.tsx";

import { Brand } from "./Brand.tsx";
import type { Insets } from "./insets.ts";
import { INSPECTOR_WIDTH, MAP_GAP, SIDEBAR_WIDTH_CLASS } from "./insets.ts";
import { useLiveMap } from "./liveMapContext.ts";
import { SECTIONS } from "./sections.ts";

interface DesktopChromeProps {
  /** The open section's body (the route), shown under the navigation. */
  children: ReactNode;
  inspectorOpen: boolean;
  onInsets: (insets: Insets) => void;
}

function SectionNav(): ReactElement {
  return (
    <nav aria-label="Sections" className="px-2">
      <ul className="flex flex-col gap-0.5">
        {SECTIONS.map((section) => (
          <li key={section.id}>
            <Link
              to={section.to}
              title={`${section.label} (${section.key})`}
              className="flex h-10 items-center gap-3 rounded-sm px-3 text-body text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent aria-[current=page]:bg-raised aria-[current=page]:text-ink"
            >
              <section.icon className="size-4" aria-hidden />
              {section.label}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}

function Inspector(): ReactElement | undefined {
  const { selected, select, now } = useLiveMap();
  const close = useCallback(() => {
    select(undefined);
  }, [select]);
  if (selected === undefined) {
    return undefined;
  }
  return (
    <aside
      aria-label="Inspector"
      className="absolute top-3 right-3 bottom-3 z-10 w-[352px] overflow-y-auto rounded-lg border border-hairline-strong shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <EventSummary event={selected} now={now} onClose={close} />
    </aside>
  );
}

/**
 * The desktop shell's chrome (≥ 1024 px, DESIGN.md §10 shells after Mac.dc.html): the sidebar with
 * the sections and the open section's body, and the inspector floating over the map's right edge.
 * The map itself is drawn by the frame, so it survives a change of shell.
 */
function DesktopChrome({ children, inspectorOpen, onInsets }: DesktopChromeProps): ReactElement {
  useLayoutEffect(() => {
    onInsets({ top: 0, right: inspectorOpen ? INSPECTOR_WIDTH + 2 * MAP_GAP : 0, bottom: 0, left: 0 });
  }, [inspectorOpen, onInsets]);

  return (
    <>
      <aside
        aria-label="Sidebar"
        className={`absolute inset-y-0 left-0 z-10 flex ${SIDEBAR_WIDTH_CLASS} flex-col border-r border-hairline bg-panel`}
      >
        <div className="px-4 pt-2 pb-3">
          <Brand />
        </div>
        <SectionNav />
        <div className="mt-3 min-h-0 flex-1 overflow-y-auto border-t border-hairline">{children}</div>
      </aside>
      {inspectorOpen ? <Inspector /> : undefined}
    </>
  );
}

export { DesktopChrome };

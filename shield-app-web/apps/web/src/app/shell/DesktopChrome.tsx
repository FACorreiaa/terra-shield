import { Link } from "@tanstack/react-router";
import type { ReactElement, ReactNode } from "react";
import { useLayoutEffect } from "react";

import { keepSelection } from "@/lib/keepSelection.ts";

import type { Insets } from "./insets.ts";
import { INSPECTOR_WIDTH, MAP_GAP, SIDEBAR_WIDTH_CLASS } from "./insets.ts";
import { MonitorHud } from "./MonitorHud.tsx";
import { LISTED_SECTIONS } from "./sections.ts";
import { SelectedDetail } from "./SelectedDetail.tsx";
import { SidebarBody } from "./SidebarBody.tsx";
import { SourcesNavRow } from "./SourcesNavRow.tsx";

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
        {LISTED_SECTIONS.map((section) => (
          <li key={section.id}>
            <Link
              to={section.to}
              search={keepSelection}
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

function Inspector(): ReactElement {
  return (
    <aside
      aria-label="Inspector"
      className="absolute top-3 right-3 bottom-3 z-10 w-[352px] overflow-y-auto rounded-lg border border-hairline-strong shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <SelectedDetail />
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
        {/* The nav mark is in the site's header above (owner, 2026-11-26). */}
        <div className="pt-3">
          <SectionNav />
        </div>
        <SidebarBody>{children}</SidebarBody>
        <SourcesNavRow />
      </aside>
      <MonitorHud />
      {inspectorOpen ? <Inspector /> : undefined}
    </>
  );
}

export { DesktopChrome };

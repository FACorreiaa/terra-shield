import type { ReactElement, ReactNode } from "react";
import { useCallback, useRef, useState } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { LiveMap } from "@/features/map/LiveMap.tsx";

import { DesktopChrome } from "./DesktopChrome.tsx";
import { MAP_LEFT_CLASS, useInsets } from "./insets.ts";
import { useLiveMap } from "./liveMapContext.ts";
import { MobileChrome } from "./MobileChrome.tsx";
import { Shortcuts } from "./Shortcuts.tsx";
import { useIsWide } from "./useBreakpoint.ts";
import { useSelectionUpkeep } from "./useDetailFocus.ts";

/**
 * The app's frame: one map, always mounted in the same place, with the desktop or the phone chrome
 * around it (DESIGN.md §10 shells, split at 1024 px), and the keyboard shortcuts.
 */
function AppFrame({ children }: { children: ReactNode }): ReactElement {
  const wide = useIsWide();
  const { list, clusters, selectedId, selectedShape, staleSources, setView, select, setMap } = useLiveMap();
  const frame = useRef<HTMLDivElement>(null);
  const applyInsets = useInsets(frame);
  useSelectionUpkeep(selectedId);
  const [inspectorWanted, setInspectorWanted] = useState(true);
  const toggleInspector = useCallback(() => {
    setInspectorWanted((was) => !was);
  }, []);
  // The inspector shows the selection; `i` hides it for those who want the map wide.
  const inspectorOpen = wide && inspectorWanted && selectedId !== undefined;

  return (
    <div className="flex h-full flex-col bg-canvas">
      <div ref={frame} className="relative min-h-0 flex-1 overflow-hidden">
        <div className={wide ? `absolute inset-y-0 right-0 ${MAP_LEFT_CLASS}` : "absolute inset-0"}>
          <LiveMap
            events={list}
            clusters={clusters}
            staleSources={staleSources}
            selectedId={selectedId}
            selectedShape={selectedShape}
            onViewChange={setView}
            onSelect={select}
            onMap={setMap}
          />
        </div>
        {wide ? (
          <DesktopChrome inspectorOpen={inspectorOpen} onInsets={applyInsets}>
            {children}
          </DesktopChrome>
        ) : (
          <MobileChrome onInsets={applyInsets}>{children}</MobileChrome>
        )}
      </div>
      {wide ? (
        <footer className="border-t border-hairline px-4 py-2">
          <DisclaimerBanner variant="footer" />
        </footer>
      ) : undefined}
      <Shortcuts toggleInspector={toggleInspector} />
    </div>
  );
}

export { AppFrame };

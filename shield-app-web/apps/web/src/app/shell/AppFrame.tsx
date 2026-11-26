import type { ReactElement, ReactNode } from "react";
import { useCallback, useMemo, useRef, useState } from "react";

import { LiveMap } from "@/features/map/LiveMap.tsx";

import { DesktopChrome } from "./DesktopChrome.tsx";
import { ShellFooter, ShellHeader } from "./frameBars.tsx";
import { MAP_LEFT_CLASS, useInsets } from "./insets.ts";
import { KeyboardShortcuts } from "./KeyboardShortcuts.tsx";
import { useLiveMap } from "./liveMapContext.ts";
import { MobileChrome } from "./MobileChrome.tsx";
import { useIsWide } from "./useBreakpoint.ts";
import { useSelectionUpkeep } from "./useDetailFocus.ts";

/**
 * The app's frame: one map, always mounted in the same place, with the desktop or the phone chrome
 * around it (DESIGN.md §10 shells, split at 1024 px), and the keyboard shortcuts.
 */
function AppFrame({ children }: { children: ReactNode }): ReactElement {
  const wide = useIsWide();
  const { drawn, clusters, selectedId, selectedShape, staleSources, setView, select, setMap } = useLiveMap();
  const { aircraftHex, selectAircraft, inspecting } = useLiveMap();
  const { arrivals, started, arrived, detections, conflicts } = useLiveMap();
  const arrival = useMemo(() => ({ onStarted: started, onArrived: arrived }), [started, arrived]);
  const frame = useRef<HTMLDivElement>(null);
  const applyInsets = useInsets(frame);
  useSelectionUpkeep(selectedId, aircraftHex);
  const [inspectorWanted, setInspectorWanted] = useState(true);
  const toggleInspector = useCallback(() => {
    setInspectorWanted((was) => !was);
  }, []);
  // The inspector shows the selection; `i` hides it for those who want the map wide.
  const inspectorOpen = wide && inspectorWanted && inspecting;

  return (
    <div className="flex h-full flex-col bg-canvas">
      {wide ? <ShellHeader /> : undefined}
      <div ref={frame} className="relative min-h-0 flex-1 overflow-hidden">
        <div className={wide ? `absolute inset-y-0 right-0 ${MAP_LEFT_CLASS}` : "absolute inset-0"}>
          <LiveMap
            events={drawn}
            clusters={clusters}
            conflictClusters={conflicts.clusters}
            staleSources={staleSources}
            selectedId={selectedId}
            selectedShape={selectedShape}
            onViewChange={setView}
            onSelect={select}
            aircraftHex={aircraftHex}
            onSelectAircraft={selectAircraft}
            onMap={setMap}
            arrivals={arrivals}
            arrival={arrival}
            detections={detections.layer}
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
      {wide ? <ShellFooter /> : undefined}
      <KeyboardShortcuts toggleInspector={toggleInspector} />
    </div>
  );
}

export { AppFrame };

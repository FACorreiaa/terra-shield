import type { ReactElement } from "react";

import { DetectionLegend } from "@/features/detections/DetectionLegend.tsx";
import { ErrorBanner } from "@/features/monitor/ErrorBanner.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useRetry } from "./useRetry.ts";

/**
 * The notices over the map, in the apps' order (MapScreen.swift): the error banner when a load
 * fails, then the key of each layer that is on. `className` goes on each, e.g. to clear the
 * phone's control column.
 */
function MapNotices({ className }: { className?: string }): ReactElement {
  const { failure, detections, now } = useLiveMap();
  const retry = useRetry();
  return (
    <>
      {failure === undefined ? undefined : (
        <div className={className}>
          <ErrorBanner message={failure} onRetry={retry} />
        </div>
      )}
      {detections.shown ? (
        <div className={className}>
          <DetectionLegend detections={detections} now={now} />
        </div>
      ) : undefined}
    </>
  );
}

export { MapNotices };

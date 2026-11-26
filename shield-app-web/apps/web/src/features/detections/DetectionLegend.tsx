import type { ReactElement } from "react";

import { detectionLegend } from "@/domain/detections.ts";
import { cn } from "@/lib/utils.ts";

import type { Detections } from "./useDetections.ts";

const PART = " · ";

interface DetectionLegendProps {
  detections: Detections;
  now: Date;
}

/**
 * The heat layer's key, under the map's notices while the layer is on (`DetectionLegend`,
 * MapScreen.swift): what the dots are and how old the newest is, "Detected 3 h ago", never
 * "Updated". A failed load says so here, not in the events' banner.
 */
function DetectionLegend({ detections, now }: DetectionLegendProps): ReactElement {
  const { layer, failed, loading } = detections;
  const text = detectionLegend(layer, { failed, loading, now });
  return (
    <div
      data-detection-legend
      className="flex w-fit max-w-full items-center gap-2 rounded-2xl border border-hairline-strong px-3 py-1.5 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <span className="size-1.5 shrink-0 rounded-full bg-kind-detection" aria-hidden />
      {/* Wraps on a narrow phone rather than cut off "Zoom in for all", as the apps' Text, between parts only. */}
      <span className={cn("text-meta", failed ? "text-warning" : "text-ink")}>
        {text.split(PART).map((part, index) => (
          <span key={part} className="whitespace-nowrap">
            {index === 0 ? part : `${PART}${part}`}
          </span>
        ))}
      </span>
      {/* The key itself stays quiet as its age ticks; a failure is announced once. */}
      <output className="sr-only">{failed ? text : ""}</output>
    </div>
  );
}

export { DetectionLegend };

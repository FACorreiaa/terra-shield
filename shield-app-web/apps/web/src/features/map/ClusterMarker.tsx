import type { EventCluster } from "@shield/api-client";
import { Crosshair } from "lucide-react";
import type { ReactElement } from "react";
import { useMemo } from "react";

import { severityShares } from "@/domain/viewport.ts";

import { CLUSTER, clusterArcs, clusterCount, clusterLabel, clusterSize } from "./clusterArt.ts";

interface ClusterMarkerProps {
  cluster: Pick<EventCluster, "count" | "maxSeverity" | "severities">;
  onActivate: () => void;
  /**
   * A cluster of armed conflicts (ADR 27): a dashed ring and a small crosshair badge, so it never
   * reads as hazards (ClusterMarker.swift `isConflict`).
   */
  conflict?: boolean;
}

/** The conflict ring's dash, px (ClusterMarker.swift: `dash: [4, 3]`). */
const CONFLICT_DASH = "4 3";

/**
 * Several events drawn as one (DESIGN.md §9): a `panel` disc, a 2 px ring in the highest severity,
 * an arc per severity proportional to its share, the count in `ink` mono. Activating it zooms to
 * the cluster's extent.
 */
function ClusterMarker({ cluster, onActivate, conflict = false }: ClusterMarkerProps): ReactElement {
  const size = clusterSize(cluster.count);
  const box = useMemo(() => ({ width: size, height: size }), [size]);
  const center = size / 2;
  const badge = useMemo(
    () => ({ backgroundColor: `var(--color-severity-${cluster.maxSeverity})` }),
    [cluster.maxSeverity],
  );
  const arcs = clusterArcs(severityShares(cluster), size);
  return (
    <button
      type="button"
      aria-label={clusterLabel(cluster.count, cluster.maxSeverity, conflict)}
      title="Zoom in to show the events"
      onClick={onActivate}
      className="relative grid cursor-pointer place-items-center rounded-full drop-shadow-[var(--shadow-marker)] transition-transform duration-(--motion-fast) ease-(--ease-shield) hover:scale-105 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
      style={box}
    >
      <svg aria-hidden="true" className="absolute inset-0" width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        <circle cx={center} cy={center} r={center} className="fill-panel" />
        <circle
          cx={center}
          cy={center}
          r={center - CLUSTER.ringWidth / 2}
          fill="none"
          strokeWidth={CLUSTER.ringWidth}
          strokeDasharray={conflict ? CONFLICT_DASH : undefined}
          stroke={`var(--color-severity-${cluster.maxSeverity})`}
        />
        {arcs.map((arc) => (
          <path
            key={arc.severity}
            d={arc.path}
            fill="none"
            strokeWidth={CLUSTER.arcWidth}
            stroke={`var(--color-severity-${arc.severity})`}
          />
        ))}
      </svg>
      <span className="relative px-1.5 font-mono text-meta font-semibold text-ink tabular-nums">
        {clusterCount(cluster.count)}
      </span>
      {conflict ? (
        <span
          aria-hidden
          className="absolute -top-0.5 -right-0.5 grid size-4 place-items-center rounded-full text-canvas"
          style={badge}
        >
          <Crosshair className="size-2.5" strokeWidth={3} />
        </span>
      ) : undefined}
    </button>
  );
}

export { ClusterMarker };

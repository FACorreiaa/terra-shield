import type { EventCluster } from "@shield/api-client";
import type { ReactElement } from "react";
import { useMemo } from "react";

import { severityShares } from "@/domain/viewport.ts";

import { CLUSTER, clusterArcs, clusterCount, clusterLabel, clusterSize } from "./clusterArt.ts";

interface ClusterMarkerProps {
  cluster: Pick<EventCluster, "count" | "maxSeverity" | "severities">;
  onActivate: () => void;
}

/**
 * Several events drawn as one (DESIGN.md §9): a `panel` disc, a 2 px ring in the highest severity,
 * an arc per severity proportional to its share, the count in `ink` mono. Activating it zooms to
 * the cluster's extent.
 */
function ClusterMarker({ cluster, onActivate }: ClusterMarkerProps): ReactElement {
  const size = clusterSize(cluster.count);
  const box = useMemo(() => ({ width: size, height: size }), [size]);
  const center = size / 2;
  const arcs = clusterArcs(severityShares(cluster), size);
  return (
    <button
      type="button"
      aria-label={clusterLabel(cluster.count, cluster.maxSeverity)}
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
    </button>
  );
}

export { ClusterMarker };

import type { Severity } from "@shield/api-client";

import { severityLabel } from "@/domain/taxonomy.ts";
import type { SeverityShare } from "@/domain/viewport.ts";

/**
 * Cluster marker geometry (DESIGN.md §9), the web twin of `ClusterMarker.swift`: a `panel` disc,
 * a 2 px ring in the highest severity, an arc per severity proportional to its share, the count
 * in mono. With no split known the ring is the maximum only; no split is guessed.
 */

const CLUSTER = {
  minSize: 36,
  growth: 20,
  /** The count at which a cluster stops growing, as a multiple of the smallest (2). */
  growthSpan: 500,
  smallestCount: 2,
  ringWidth: 2,
  arcWidth: 4,
  arcInset: 5,
} as const;

interface Arc {
  severity: Severity;
  /** SVG path for the arc, in a box of side `size`. */
  path: string;
}

/** Grows with the count, slowly: 36 px for 2 events, 56 px from about a thousand. */
function clusterSize(count: number): number {
  const growth =
    Math.log10(Math.max(count, CLUSTER.smallestCount) / CLUSTER.smallestCount) / Math.log10(CLUSTER.growthSpan);
  return CLUSTER.minSize + CLUSTER.growth * Math.min(1, growth);
}

function pointOn(center: number, radius: number, fraction: number): string {
  const angle = fraction * Math.PI * 2 - Math.PI / 2;
  return `${(center + radius * Math.cos(angle)).toFixed(2)} ${(center + radius * Math.sin(angle)).toFixed(2)}`;
}

/**
 * One arc per severity, clockwise from twelve o'clock, most severe first. A single-severity
 * cluster is already the ring, so it gets no arcs.
 */
function clusterArcs(shares: readonly SeverityShare[], size: number): Arc[] {
  if (shares.length <= 1) {
    return [];
  }
  const center = size / 2;
  const radius = center - CLUSTER.arcInset - CLUSTER.arcWidth / 2;
  let start = 0;
  return shares.map(({ severity, share }) => {
    const end = start + share;
    const large = share > 1 / 2 ? 1 : 0;
    const path = `M ${pointOn(center, radius, start)} A ${radius} ${radius} 0 ${large} 1 ${pointOn(center, radius, end)}`;
    start = end;
    return { severity, path };
  });
}

const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });

/** "1.2K", "77". */
function clusterCount(count: number): string {
  return compact.format(count);
}

/** Screen-reader name: "Cluster, 14 events, highest severity extreme". */
function clusterLabel(count: number, maxSeverity: Severity): string {
  return `Cluster, ${count} events, highest severity ${severityLabel(maxSeverity).toLowerCase()}`;
}

export type { Arc };
export { CLUSTER, clusterArcs, clusterCount, clusterLabel, clusterSize };

import type { Severity } from "@shield/api-client";
import type { ReactElement } from "react";

import { Icon } from "@/design/icons.ts";
import { severityTokens } from "@/design/tokens.gen.ts";
import { cn } from "@/lib/utils.ts";

const fill: Record<Severity, string> = {
  minor: "bg-severity-minor",
  moderate: "bg-severity-moderate",
  severe: "bg-severity-severe",
  extreme: "bg-severity-extreme",
};

interface SeverityBadgeProps {
  severity: Severity;
  /** The source's own value, shown verbatim in muted; empty or absent shows nothing. */
  sourceSeverity?: string | undefined;
  /** Derived from a weak signal (DESIGN.md §5). */
  provisional?: boolean | undefined;
}

/** Shield's severity as shape + word + colour, then the source's value verbatim (DESIGN.md §4). */
function SeverityBadge({ severity, sourceSeverity = "", provisional = false }: SeverityBadgeProps): ReactElement {
  const token = severityTokens[severity];
  return (
    <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-meta">
      <span
        className={cn(
          "inline-flex shrink-0 items-center gap-1 rounded-xs px-2 py-0.5 font-semibold text-canvas",
          fill[severity],
        )}
      >
        <Icon name={token.icon} aria-hidden className="size-3" fill="currentColor" strokeWidth={2.5} />
        {token.label}
      </span>
      {provisional ? <span className="text-muted italic">Provisional</span> : undefined}
      {sourceSeverity === "" ? undefined : <span className="text-muted">{sourceSeverity}</span>}
    </span>
  );
}

export { SeverityBadge };

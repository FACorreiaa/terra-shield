import type { EventKind } from "@shield/api-client";
import type { ReactElement } from "react";

import { Icon } from "@/design/icons.ts";
import { kindTokens } from "@/design/tokens.gen.ts";
import { cn } from "@/lib/utils.ts";

const kindIcon: Record<EventKind, string> = {
  observed: "eye",
  warning: "badge-check",
  forecast: "clock",
  detection: "thermometer",
};

/** What sort of record this is, in fixed words. Forecasts get a dashed border (DESIGN.md §5). */
function KindChip({ kind }: { kind: EventKind }): ReactElement {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-sm border bg-raised px-2 py-0.5 text-meta text-ink",
        kind === "forecast" ? "border-dashed border-muted" : "border-ink/10",
      )}
    >
      <Icon name={kindIcon[kind]} aria-hidden className="size-3.5 text-muted" />
      {kindTokens[kind].label}
    </span>
  );
}

export { KindChip };

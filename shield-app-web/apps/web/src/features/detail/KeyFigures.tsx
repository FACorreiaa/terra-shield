import type { ReactElement } from "react";

import type { KeyFigure } from "@/domain/eventFacts.ts";

/**
 * Type-specific facts from `attrs`, only those present (DESIGN.md §10.3 item 4). The lead figure
 * is large; the rest are label and value.
 */
function KeyFigures({ figures }: { figures: readonly KeyFigure[] }): ReactElement | undefined {
  if (figures.length === 0) {
    return undefined;
  }
  const lead = figures.find((figure) => figure.isLead);
  const rest = figures.filter((figure) => !figure.isLead);
  return (
    <section aria-label="Key figures" className="flex flex-col gap-3">
      {lead === undefined ? undefined : (
        <p className="flex flex-col">
          <span className="truncate text-stat-hero text-ink">{lead.value}</span>
          <span className="text-meta text-muted">{lead.label}</span>
        </p>
      )}
      {rest.length === 0 ? undefined : (
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-meta">
          {rest.map((figure) => (
            <div key={figure.label} className="contents">
              <dt className="text-muted">{figure.label}</dt>
              <dd className="text-ink">{figure.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  );
}

export { KeyFigures };

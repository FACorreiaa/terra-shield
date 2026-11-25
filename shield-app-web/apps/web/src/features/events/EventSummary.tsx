import type { HazardEvent } from "@shield/api-client";
import { hazardType } from "@shield/api-client";
import { ExternalLink, X } from "lucide-react";
import type { ReactElement } from "react";

import { Icon } from "@/design/icons.ts";
import { hazardTypeTokens } from "@/design/tokens.gen.ts";
import { place } from "@/domain/place.ts";

import { FreshnessLabel } from "./FreshnessLabel.tsx";
import { KindChip } from "./KindChip.tsx";
import { SeverityBadge } from "./SeverityBadge.tsx";

interface EventSummaryProps {
  event: HazardEvent;
  now: Date;
  onClose: () => void;
}

/**
 * The selected event's essentials in the inspector or the sheet: what, where, both severities, kind,
 * source and freshness, and the way to the official record. The full detail (DESIGN.md §10.3)
 * replaces it in W5.
 */
function EventSummary({ event, now, onClose }: EventSummaryProps): ReactElement {
  const typeToken = hazardTypeTokens[hazardType(event.type)];
  const where = place(event);
  return (
    <article aria-label={event.title} className="flex flex-col gap-3 p-4">
      <header className="flex items-start gap-3">
        <Icon name={typeToken.icon} aria-label={typeToken.label} className="mt-1 size-5 shrink-0 text-muted" />
        <h2 className="min-w-0 flex-1 text-title text-ink">{event.title}</h2>
        <button
          type="button"
          aria-label="Close"
          title="Close (Esc)"
          onClick={onClose}
          className="-mt-2 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
        >
          <X className="size-4" aria-hidden />
        </button>
      </header>
      {where === undefined ? undefined : <p className="text-body text-muted">{where}</p>}
      <SeverityBadge
        severity={event.severity}
        sourceSeverity={event.sourceSeverity}
        provisional={event.severityProvisional}
      />
      <p className="flex flex-wrap items-center gap-2">
        <KindChip kind={event.kind} />
        <span className="text-meta text-muted">{event.source.toUpperCase()}</span>
      </p>
      <FreshnessLabel updatedAt={event.updatedAt} now={now} />
      {event.url === undefined ? undefined : (
        <a
          href={event.url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex h-11 items-center gap-2 self-start rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
        >
          Open official source
          <ExternalLink className="size-4" aria-hidden />
        </a>
      )}
    </article>
  );
}

export { EventSummary };

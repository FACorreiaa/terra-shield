import { CircleHelp, ClockAlert } from "lucide-react";
import type { ReactElement } from "react";

import { relative } from "@/domain/freshness.ts";

interface BannerProps {
  tone: "warning" | "muted";
  children: string;
}

const TONE = { warning: "text-warning", muted: "text-muted" } as const;

function Banner({ tone, children }: BannerProps): ReactElement {
  const Icon = tone === "warning" ? ClockAlert : CircleHelp;
  return (
    <p className={`flex items-start gap-2 rounded-sm border border-hairline bg-raised/50 p-3 text-meta ${TONE[tone]}`}>
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
      {children}
    </p>
  );
}

/** The event's source has missed three polls (DECISIONS #8). */
function StaleBanner({ lastUpdate, now }: { lastUpdate: Date; now: Date }): ReactElement {
  return (
    <Banner tone="warning">
      {`Stale · this source last updated ${relative(lastUpdate, now)}. The event may have changed since.`}
    </Banner>
  );
}

/** A 404, or a link to an event its source has dropped (DESIGN.md §10.3 "Gone"). */
function GoneBanner({ sourceName }: { sourceName: string | undefined }): ReactElement {
  return (
    <Banner tone="muted">
      {`This event is no longer published by its source. Open official source goes to ${sourceName ?? "the source"}’s home page.`}
    </Banner>
  );
}

export { Banner, GoneBanner, StaleBanner };

import type { Source } from "@shield/api-client";
import { ExternalLink } from "lucide-react";
import type { ReactElement } from "react";

import { stamp } from "@/domain/freshness.ts";

/** The host, for the tooltip; an upstream URL that doesn't parse gets none rather than a crash. */
function hostOf(url: string): string | undefined {
  return URL.canParse(url) ? new URL(url).host : undefined;
}

/**
 * The primary action of an event (DESIGN.md §10.3 item 6): accent fill, `on-accent` text, `o`. Opens
 * the source's own record in a new tab.
 */
function OfficialSourceButton({ url }: { url: string }): ReactElement {
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      title={["Open official source (o)", hostOf(url)].filter(Boolean).join(" · ")}
      className="inline-flex min-h-11 items-center gap-2 self-start rounded-sm bg-accent px-4 py-2.5 text-body font-semibold text-on-accent hover:bg-accent/90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
    >
      Open official source
      <ExternalLink className="size-4" aria-hidden />
    </a>
  );
}

interface SourceBadgeProps {
  source: Source;
  /** Where "Open official source" goes (`officialUrl`). */
  url: string | undefined;
  /** When Shield last polled the source: not the event's `fetchedAt`, which moves only on change. */
  checkedAt: Date | undefined;
  now: Date;
}

/** Who said so (DECISIONS #11), in full: name, attribution and licence, last check, the record. */
function SourceBadge({ source, url, checkedAt, now }: SourceBadgeProps): ReactElement {
  return (
    <section aria-label="Source" className="flex flex-col gap-1">
      <h3 className="text-micro tracking-[0.08em] text-faint uppercase">Source</h3>
      <p className="text-headline text-ink">{source.name}</p>
      <p className="text-meta text-muted">{`${source.attribution} · ${source.license}`}</p>
      {checkedAt === undefined ? undefined : (
        <p className="font-mono text-meta text-muted tabular-nums">{`Checked ${stamp(checkedAt, now)}`}</p>
      )}
      {url === undefined ? undefined : (
        <div className="pt-2">
          <OfficialSourceButton url={url} />
        </div>
      )}
    </section>
  );
}

export { OfficialSourceButton, SourceBadge };

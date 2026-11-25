import type { Source } from "@shield/api-client";
import { ExternalLink } from "lucide-react";
import type { ReactElement } from "react";

import type { SourceHealth } from "@/domain/sources.ts";
import { health, interval, statusLine } from "@/domain/sources.ts";

import { HealthDot } from "./HealthDot.tsx";

const STATUS_COLOUR: Readonly<Record<SourceHealth, string>> = {
  ok: "text-muted",
  warning: "text-warning",
  danger: "text-danger",
};

interface OutLinkProps {
  href: string;
  children: string;
  /** The source's name: every row has a "Website", so screen readers hear whose. */
  name: string;
}

function OutLink({ href, children, name }: OutLinkProps): ReactElement | undefined {
  if (!URL.canParse(href)) {
    return undefined;
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex min-h-11 items-center gap-1.5 rounded-sm text-meta font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
    >
      <ExternalLink aria-hidden className="size-3.5" />
      {children}
      <span className="sr-only">{`(${name}, opens in a new tab)`}</span>
    </a>
  );
}

/**
 * One source (SourceRow.swift, DESIGN.md §10.7): name with its health dot, what it covers, its last
 * check, how often we check it, licence and attribution, then its terms and website. Without a
 * terms page the one link is the website.
 */
function SourceRow({ source, now }: { source: Source; now: Date }): ReactElement {
  const state = health(source, now);
  return (
    <li className="flex flex-col gap-1.5 py-3">
      <h3 className="flex items-baseline gap-2 text-headline text-ink">
        <HealthDot health={state} />
        {source.name}
      </h3>
      {source.coverage === undefined || source.coverage === "" ? undefined : (
        <p className="text-body text-ink">{source.coverage}</p>
      )}
      <p className={`text-meta ${STATUS_COLOUR[state]}`}>{statusLine(source, now)}</p>
      <p className="text-meta text-muted">{interval(source)}</p>
      <p className="text-meta text-muted">{`Licence: ${source.license}`}</p>
      <p className="text-meta text-muted">{`Attribution: ${source.attribution}`}</p>
      <div className="flex flex-wrap gap-x-4">
        {source.termsUrl === undefined ? (
          <OutLink href={source.url} name={source.name}>
            Website
          </OutLink>
        ) : (
          <>
            <OutLink href={source.termsUrl} name={source.name}>
              Terms
            </OutLink>
            <OutLink href={source.url} name={source.name}>
              Website
            </OutLink>
          </>
        )}
      </div>
    </li>
  );
}

export { SourceRow };

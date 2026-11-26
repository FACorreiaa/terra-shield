import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { listed } from "@/domain/sources.ts";
import { useSources } from "@/features/sources/useSources.ts";

/**
 * The official sources by name, with the attribution each asks for, as the Sources screen lists
 * them. Nothing while they load or if they fail: the names are never written into the page.
 */
function SourcesSection(): ReactElement | undefined {
  const { data } = useSources();
  const sources = data === undefined ? [] : listed(data);
  if (sources.length === 0) {
    return undefined;
  }
  return (
    <section aria-labelledby="sources-heading" className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-16">
      <h2 id="sources-heading" className="text-title text-ink">
        Sources
      </h2>
      <ul className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
        {sources.map((source) => (
          <li key={source.id}>
            {/* Name and attribution are one target, so every row is as tall as its words, and at least 44 px. */}
            <a
              href={source.url}
              target="_blank"
              rel="noreferrer"
              className="group flex min-h-11 flex-col justify-center gap-0.5 rounded-sm py-1 focus-visible:outline-2 focus-visible:outline-accent"
            >
              <span className="text-body text-ink underline-offset-4 group-hover:underline">{source.name}</span>
              <span className="text-micro text-muted">{source.attribution}</span>
            </a>
          </li>
        ))}
      </ul>
      <Link
        to="/sources"
        className="flex min-h-11 items-center self-start rounded-sm text-body text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-accent"
      >
        How each source is doing
      </Link>
    </section>
  );
}

export { SourcesSection };

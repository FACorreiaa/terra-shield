import type { Source } from "@shield/api-client";
import { TriangleAlert } from "lucide-react";
import type { ReactElement } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { listed } from "@/domain/sources.ts";

import { SourceRow } from "./SourceRow.tsx";

interface SourcesViewProps {
  /** The last list loaded; undefined before the first. */
  sources: readonly Source[] | undefined;
  /** The last request failed (a refresh, or the first load). */
  failed: boolean;
  onRetry: () => void;
}

/** Nothing listed yet: loading, or why not. The failure stays up while Try again runs, so its button keeps focus. */
function Unlisted({ failed, onRetry }: Omit<SourcesViewProps, "sources">): ReactElement {
  if (!failed) {
    return <p className="text-body text-muted">Loading sources…</p>;
  }
  return (
    <div className="flex flex-col items-start gap-1">
      <p role="alert" className="text-body text-ink">
        Couldn’t load the sources.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="inline-flex min-h-11 items-center rounded-sm text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
      >
        Try again
      </button>
    </div>
  );
}

function SourceList({
  sources,
  failed,
  now,
}: {
  sources: readonly Source[];
  failed: boolean;
  now: Date;
}): ReactElement {
  return (
    <>
      {failed ? (
        <p role="alert" className="flex items-start gap-1.5 text-meta text-warning">
          <TriangleAlert aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          Couldn’t refresh. Showing the last list loaded.
        </p>
      ) : undefined}
      <section aria-labelledby="sources-count">
        <h2 id="sources-count" className="text-micro tracking-[0.08em] text-muted uppercase">
          {`${sources.length} sources`}
        </h2>
        <ul className="flex flex-col divide-y divide-hairline">
          {sources.map((source) => (
            <SourceRow key={source.id} source={source} now={now} />
          ))}
        </ul>
      </section>
      <p className="pt-2 text-meta text-muted">Absence of data is not safety.</p>
    </>
  );
}

/**
 * Sources & attribution (SourcesView.swift, DESIGN.md §10.7): the disclaimer, then every active
 * source with its health. A failed refresh keeps the last list and says so; an empty answer is
 * treated as a failed one, since every source is meant to be listed.
 */
function SourcesView({ sources, failed, onRetry, now }: SourcesViewProps & { now: Date }): ReactElement {
  const shown = listed(sources ?? []);
  return (
    <div className="flex flex-col gap-4 px-4 py-4">
      <DisclaimerBanner variant="inline" />
      {shown.length === 0 ? (
        <Unlisted failed={failed || sources !== undefined} onRetry={onRetry} />
      ) : (
        <SourceList sources={shown} failed={failed} now={now} />
      )}
    </div>
  );
}

export { SourcesView };

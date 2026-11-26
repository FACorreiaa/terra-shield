import type { HazardEvent, Source } from "@shield/api-client";
import type { ReactElement } from "react";

import { countryCells } from "@/domain/countryOverview.ts";
import { StatStrip } from "@/features/monitor/StatStrip.tsx";

import { CountryBody } from "./CountryBody.tsx";
import { CountryHeader } from "./CountryHeader.tsx";
import type { CountryState } from "./countryState.ts";

interface CountryViewProps {
  name: string;
  code: string | undefined;
  state: CountryState;
  sources: readonly Source[] | undefined;
  selectedId: string | undefined;
  now: Date;
  onSelect: (event: HazardEvent) => void;
  onClose: () => void;
  onRetry: () => void;
}

const ACTION =
  "inline-flex min-h-11 items-center self-start rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent";

function Unloaded({ state, onRetry }: { state: CountryState; onRetry: () => void }): ReactElement | undefined {
  if (state.kind === "notFound") {
    return (
      <div className="flex flex-col gap-1">
        <p className="text-body text-ink">We have no boundary for this place.</p>
        <p className="text-meta text-muted">Aegis Terra can only summarise countries it has borders for.</p>
      </div>
    );
  }
  if (state.kind === "failed") {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-body text-danger">The country couldn’t be loaded. Check your connection and try again.</p>
        <button type="button" onClick={onRetry} className={ACTION}>
          Retry
        </button>
      </div>
    );
  }
  return undefined;
}

/**
 * One country's overview (DESIGN.md §10.2, CountryView.swift): what is happening there, how
 * serious, and how well it is covered. Its events ignore the map's filters: all of it.
 */
function CountryView(props: CountryViewProps): ReactElement {
  const { name, code, state, sources, selectedId, now, onSelect, onClose, onRetry } = props;
  return (
    <article aria-label={name} aria-busy={state.kind === "loading"} className="flex flex-col gap-5 px-4 py-4">
      <CountryHeader name={name} code={code} state={state} now={now} onClose={onClose} />
      {state.kind === "loaded" ? (
        <>
          <StatStrip cells={countryCells(state.summary)} />
          <CountryBody
            summary={state.summary}
            sources={sources}
            selectedId={selectedId}
            onSelect={onSelect}
            now={now}
          />
        </>
      ) : (
        <Unloaded state={state} onRetry={onRetry} />
      )}
    </article>
  );
}

export { CountryView };

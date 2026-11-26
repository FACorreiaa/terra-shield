import { X } from "lucide-react";
import type { ReactElement } from "react";

import { relative } from "@/domain/freshness.ts";

import type { CountryState } from "./countryState.ts";

/** Focusable from script, so opening a country puts keyboard focus on its name. */
const COUNTRY_HEADING = "data-country-heading";

/** What changed, spoken: loading, and a refresh that failed. The minute's "Updated" stays quiet. */
function spoken(state: CountryState, age: string | undefined): string | undefined {
  if (state.kind === "loading") {
    return "Loading…";
  }
  return state.kind === "loaded" && state.refreshFailed
    ? `The country couldn’t be refreshed. Showing data from ${age ?? ""}.`
    : undefined;
}

function Status({ state, now }: { state: CountryState; now: Date }): ReactElement {
  const age = state.kind === "loaded" ? relative(new Date(state.summary.generatedAt), now) : undefined;
  const said = spoken(state, age);
  return (
    <>
      <output className={`block text-meta ${state.kind === "loading" ? "text-muted" : "text-warning"}`}>{said}</output>
      {said === undefined && age !== undefined ? <p className="text-meta text-muted">{`Updated ${age}`}</p> : undefined}
    </>
  );
}

interface CountryHeaderProps {
  name: string;
  /** Shown beside the name when it differs (a code with no known name shows once). */
  code: string | undefined;
  state: CountryState;
  now: Date;
  onClose: () => void;
}

/** The country's name, its ISO code in mono, how fresh the numbers are, and Close (DESIGN.md §10.2 item 1). */
function CountryHeader({ name, code, state, now, onClose }: CountryHeaderProps): ReactElement {
  return (
    <header className="flex flex-col gap-1">
      <div className="flex items-start gap-2">
        <h2 tabIndex={-1} data-country-heading className="min-w-0 flex-1 pt-2 text-title text-ink outline-none">
          {name}
          {code === undefined || code === name ? undefined : (
            <span aria-hidden className="ml-2 font-mono text-meta text-muted">
              {code}
            </span>
          )}
        </h2>
        <button
          type="button"
          aria-label={`Close ${name}`}
          title="Close (Esc)"
          onClick={onClose}
          className="-mr-2 flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:bg-ink/8 hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>
      <Status state={state} now={now} />
    </header>
  );
}

export { COUNTRY_HEADING, CountryHeader };

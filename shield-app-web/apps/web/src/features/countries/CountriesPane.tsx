import { Link } from "@tanstack/react-router";
import { ListFilter, X } from "lucide-react";
import type { ReactElement } from "react";
import { useCallback, useId, useMemo, useState } from "react";

import { Icon } from "@/design/icons.ts";
import { severityTokens } from "@/design/tokens.gen.ts";
import type { Country } from "@/domain/countries.ts";
import { activeWords, directory } from "@/domain/countryOverview.ts";

const SHAPE = {
  minor: "text-severity-minor",
  moderate: "text-severity-moderate",
  severe: "text-severity-severe",
  extreme: "text-severity-extreme",
} as const;

function Row({ country }: { country: Country }): ReactElement {
  const worst = country.activeEvents > 0 ? country.maxSeverity : undefined;
  const params = useMemo(() => ({ iso2: country.iso2 }), [country.iso2]);
  return (
    <li>
      <Link
        to="/countries/$iso2"
        params={params}
        data-country-row={country.iso2}
        className="flex min-h-11 items-center gap-2 px-4 text-body text-ink hover:bg-raised/60 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent"
      >
        <span aria-hidden className="min-w-0 flex-1 truncate">
          {country.name}
        </span>
        {/* "Chile, 3 active events in or near it, up to severe" for screen readers. */}
        <span className="sr-only">{activeWords(country)}</span>
        {worst === undefined ? undefined : (
          <>
            <Icon
              name={severityTokens[worst].icon}
              aria-hidden
              className={`size-3 ${SHAPE[worst]}`}
              fill="currentColor"
            />
            <span aria-hidden className="font-mono text-meta text-muted tabular-nums">
              {country.activeEvents}
            </span>
          </>
        )}
      </Link>
    </li>
  );
}

function Group({ title, countries }: { title: string; countries: readonly Country[] }): ReactElement | undefined {
  const id = useId();
  if (countries.length === 0) {
    return undefined;
  }
  return (
    <section aria-labelledby={id} className="flex flex-col">
      <h2 id={id} className="px-4 pt-3 pb-1 text-micro tracking-[0.08em] text-muted uppercase">
        {title}
      </h2>
      <ul>
        {countries.map((country) => (
          <Row key={country.iso2} country={country} />
        ))}
      </ul>
    </section>
  );
}

interface CountriesPaneProps {
  countries: readonly Country[] | undefined;
  failed: boolean;
  onRetry: () => void;
}

function FilterField({ value, onChange }: { value: string; onChange: (value: string) => void }): ReactElement {
  const clear = useCallback(() => {
    onChange("");
  }, [onChange]);
  const change = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      onChange(event.target.value);
    },
    [onChange],
  );
  return (
    <div className="flex h-11 items-center gap-2 rounded-sm bg-ink/8 pl-3 text-body focus-within:outline-2 focus-within:outline-accent">
      <ListFilter aria-hidden className="size-4 shrink-0 text-muted" />
      <input
        type="search"
        value={value}
        onChange={change}
        placeholder="Filter countries"
        aria-label="Filter countries"
        data-country-filter
        autoComplete="off"
        className="h-full min-w-0 flex-1 bg-transparent text-[1rem] text-ink outline-none placeholder:text-muted [&::-webkit-search-cancel-button]:appearance-none"
      />
      {value === "" ? undefined : (
        <button
          type="button"
          onClick={clear}
          aria-label="Clear filter"
          className="flex size-11 items-center justify-center text-muted hover:text-ink"
        >
          <X aria-hidden className="size-4" />
        </button>
      )}
    </div>
  );
}

/** The list, or why there is none: loading, failed, or nothing matching the filter. */
function PaneBody({ countries, failed, onRetry, filter }: CountriesPaneProps & { filter: string }): ReactElement {
  const listed = directory(countries ?? [], filter);
  // An empty answer is as useless as a failed one: say so, and offer Retry.
  const none = failed || countries?.length === 0;
  if (countries === undefined || countries.length === 0) {
    return (
      <div className="flex flex-col gap-2 px-4 py-4">
        <p className="text-body text-muted">{none ? "Couldn’t load the countries." : "Loading countries…"}</p>
        {none ? (
          <button
            type="button"
            onClick={onRetry}
            className="inline-flex min-h-11 items-center self-start rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
          >
            Retry
          </button>
        ) : undefined}
      </div>
    );
  }
  if (listed.active.length === 0 && listed.quiet.length === 0) {
    return <p className="px-4 py-4 text-body text-muted">{`No country matches “${filter}”.`}</p>;
  }
  return (
    <>
      <Group title="Active events" countries={listed.active} />
      <Group title="No active events" countries={listed.quiet} />
    </>
  );
}

/**
 * The Countries section (Sidebar.swift `CountriesPane`): every country, those with active events
 * first, with a filter. Choosing one opens its overview.
 */
function CountriesPane({ countries, failed, onRetry }: CountriesPaneProps): ReactElement {
  const [filter, setFilter] = useState("");
  return (
    <div className="flex flex-col">
      <div className="border-b border-hairline px-3 py-3">
        <FilterField value={filter} onChange={setFilter} />
      </div>
      <PaneBody countries={countries} failed={failed} onRetry={onRetry} filter={filter} />
    </div>
  );
}

export { CountriesPane };

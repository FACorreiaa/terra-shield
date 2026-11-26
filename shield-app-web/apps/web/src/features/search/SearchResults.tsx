import type { HazardEvent } from "@shield/api-client";
import { Flag } from "lucide-react";
import type { ReactElement, ReactNode } from "react";
import { useCallback, useId } from "react";

import type { Country } from "@/domain/countries.ts";
import { activeEventWords, SEARCH_WORDS } from "@/domain/search.ts";
import { EventList } from "@/features/events/EventList.tsx";
import { SeverityBadge } from "@/features/events/SeverityBadge.tsx";

interface SearchResultsProps {
  text: string;
  countries: readonly Country[];
  events: readonly HazardEvent[];
  /** The events in view have loaded, so "no match" is true and not just "not yet". */
  eventsLoaded: boolean;
  /** Events grouped on the map, which search can't see. */
  clustered: number;
  selectedId: string | undefined;
  now: Date;
  onEvent: (id: string) => void;
  onCountry: (country: Country) => void;
}

function Hint({ children }: { children: ReactNode }): ReactElement {
  return <p className="px-4 py-2 text-meta text-muted">{children}</p>;
}

function Group({ title, children }: { title: string; children: ReactNode }): ReactElement {
  const id = useId();
  return (
    <section aria-labelledby={id} className="pt-3">
      <h2 id={id} className="px-4 pb-1 text-micro text-muted uppercase">
        {title}
      </h2>
      {children}
    </section>
  );
}

function CountryRow({ country, onCountry }: { country: Country; onCountry: (country: Country) => void }): ReactElement {
  const choose = useCallback(() => {
    onCountry(country);
  }, [onCountry, country]);
  return (
    <li>
      <button
        type="button"
        onClick={choose}
        title="Moves the map to the country"
        className="flex min-h-11 w-full items-center gap-3 px-4 py-2 text-left hover:bg-raised/60 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent"
      >
        <Flag aria-hidden className="size-5 shrink-0 text-muted" />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-body text-ink">{country.name}</span>
          <span className="text-meta text-muted">{activeEventWords(country.activeEvents)}</span>
        </span>
        {country.maxSeverity === undefined ? undefined : <SeverityBadge severity={country.maxSeverity} />}
      </button>
    </li>
  );
}

/**
 * What search finds (MapSearch.swift `MapSearchResults`): countries, then events in view. There are
 * no places on the web: no geocoder is wired (VIEWS.md, web deviations). A country list that failed
 * to load shows nothing rather than reading as "no such country".
 */
function SearchResults(props: SearchResultsProps): ReactElement {
  const { text, countries, events, eventsLoaded, clustered, selectedId, now, onEvent, onCountry } = props;
  if (text.trim() === "") {
    return <Hint>{SEARCH_WORDS.hint}</Hint>;
  }
  return (
    <div className="pb-3">
      {countries.length === 0 ? undefined : (
        <Group title="Countries">
          <ul>
            {countries.map((country) => (
              <CountryRow key={country.iso2} country={country} onCountry={onCountry} />
            ))}
          </ul>
        </Group>
      )}
      <Group title="Events in view">
        {eventsLoaded && events.length === 0 ? <Hint>{SEARCH_WORDS.noEvent}</Hint> : undefined}
        <EventList events={events} selectedId={selectedId} onSelect={onEvent} now={now} />
        {clustered > 0 ? <Hint>{SEARCH_WORDS.clustered}</Hint> : undefined}
      </Group>
    </div>
  );
}

export { SearchResults };

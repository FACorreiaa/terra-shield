import type { HazardEvent } from "@shield/api-client";
import { Link } from "@tanstack/react-router";
import { MapPin } from "lucide-react";
import type { ReactElement } from "react";
import { Fragment, useMemo } from "react";

import { phrase } from "@/domain/place.ts";

type EventCountry = HazardEvent["countries"][number];

/** Opening a country closes the event, as in the apps. */
const NO_EVENT = {};

function CountryLink({ country }: { country: EventCountry }): ReactElement {
  const params = useMemo(() => ({ iso2: country.iso2 }), [country.iso2]);
  return (
    <Link
      to="/countries/$iso2"
      params={params}
      search={NO_EVENT}
      // The padding widens the tap target past the line without moving the text.
      className="py-3 text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
    >
      {phrase(country)}
    </Link>
  );
}

/**
 * "Portugal, Spain" or "Off the coast of Chile, 38 km" (DESIGN.md §15): every country, none picked,
 * one line that wraps; each name opens its overview.
 */
function CountryLinks({ countries }: { countries: readonly EventCountry[] }): ReactElement | undefined {
  if (countries.length === 0) {
    return undefined;
  }
  return (
    <p className="flex items-start gap-1.5 text-meta text-muted">
      <MapPin className="mt-0.5 size-3.5 shrink-0" aria-hidden />
      <span>
        {countries.map((country, index) => (
          <Fragment key={country.iso2}>
            {index > 0 ? ", " : undefined}
            <CountryLink country={country} />
          </Fragment>
        ))}
      </span>
    </p>
  );
}

export { CountryLinks };

import { createFileRoute, useParams } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { Fragment, useCallback } from "react";

import { useLiveMap } from "@/app/shell/liveMapContext.ts";
import { CountryOpened, useCountryArrival } from "@/app/shell/useCountryArrival.ts";
import { useCloseCountry, useReopens } from "@/app/shell/usePage.ts";
import { countryName } from "@/domain/place.ts";
import { CountryView } from "@/features/countries/CountryView.tsx";
import { useCountries } from "@/features/countries/useCountries.ts";
import { useCountrySummary } from "@/features/countries/useCountrySummary.ts";

/**
 * A country's overview (`/countries/PT`), in the sidebar on a wide screen and the sheet on a phone.
 * Close goes up to the Countries list; Back goes back.
 */
function CountryRoute(): ReactElement {
  const { iso2 } = useParams({ from: "/_app/countries/$iso2" });
  const { code, state, retry } = useCountrySummary(iso2);
  const known = useCountries(true).data?.find((country) => country.iso2 === code);
  const { sources, selectedId, choose, now } = useLiveMap();
  const closeCountry = useCloseCountry();
  const close = useCallback(() => {
    closeCountry(code);
  }, [closeCountry, code]);
  // Each arrival: a new country, or the open one chosen again from search.
  const arrival = `${code ?? iso2}#${useReopens()}`;
  useCountryArrival(arrival, state, known);
  let name = code === undefined ? iso2.toUpperCase() : (known?.name ?? countryName(code));
  if (state.kind === "loaded") {
    ({ name } = state.summary.country);
  }
  return (
    <Fragment key={arrival}>
      <CountryOpened />
      <CountryView
        name={name}
        code={code}
        state={state}
        sources={sources}
        selectedId={selectedId}
        now={now}
        onSelect={choose}
        onClose={close}
        onRetry={retry}
      />
    </Fragment>
  );
}

export const Route = createFileRoute("/_app/countries/$iso2")({ component: CountryRoute });

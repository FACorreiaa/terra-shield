import { createFileRoute } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { useIsWide } from "@/app/shell/useBreakpoint.ts";
import { CountriesPane } from "@/features/countries/CountriesPane.tsx";
import { useCountries } from "@/features/countries/useCountries.ts";

/** The Countries section: every country with its active count; its counts are refreshed on each visit. */
function CountriesSection(): ReactElement {
  const countries = useCountries(true, { fresh: true });
  const wide = useIsWide();
  const { refetch } = countries;
  const retry = useCallback(() => {
    void refetch();
  }, [refetch]);
  return (
    <section aria-label="Countries">
      {wide ? <h1 className="px-4 pt-3 text-title text-ink">Countries</h1> : undefined}
      <CountriesPane countries={countries.data} failed={countries.isError && !countries.isFetching} onRetry={retry} />
    </section>
  );
}

export const Route = createFileRoute("/_app/countries/")({ component: CountriesSection });

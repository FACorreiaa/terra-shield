import type { CountrySummary } from "@/domain/countries.ts";

/** What `GET /v1/countries/{iso2}/summary` answered: the summary, or no boundary for the code (404/400). */
type SummaryAnswer = { found: true; summary: CountrySummary } | { found: false };

/**
 * A country's overview state (CountryOverview `Phase`). A summary loaded before is kept through a
 * failed refresh, shown with its age (`refreshFailed`); only a first load that failed is a failure.
 */
type CountryState =
  | { kind: "loading" }
  | { kind: "loaded"; summary: CountrySummary; refreshFailed: boolean }
  | { kind: "notFound" }
  | { kind: "failed" };

/** `code` is undefined when the address isn't two letters: no boundary, and no request. */
function countryState(code: string | undefined, answer: SummaryAnswer | undefined, failed: boolean): CountryState {
  if (code === undefined || answer?.found === false) {
    return { kind: "notFound" };
  }
  if (answer?.found === true) {
    return { kind: "loaded", summary: answer.summary, refreshFailed: failed };
  }
  return failed ? { kind: "failed" } : { kind: "loading" };
}

export type { CountryState, SummaryAnswer };
export { countryState };

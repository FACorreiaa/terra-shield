/**
 * A country's overview and the Countries section (DESIGN.md §10.2, §10 sidebar). Mirrors
 * shield-core `CountryDirectory` (Sidebar.swift), `CountryOverview` and the strings of
 * shield-apple `CountryView`.
 */
import { SEVERITY_ORDER } from "@shield/api-client";

import type { Country, CountrySummary, MatrixCell } from "./countries.ts";
import { countIn, highSeverityCount, isListCapped, matching, warningCount } from "./countries.ts";
import type { StatCell } from "./statStrip.ts";
import { severityLabel, typeLabel } from "./taxonomy.ts";

/** The Countries section: countries with active events, then the rest. */
interface CountryDirectory {
  active: Country[];
  quiet: Country[];
}

/** Below every severity: a country with events but no worst severity sorts last among the active. */
const NO_RANK = -1;

function byName(left: Country, right: Country): number {
  return left.name.localeCompare(right.name, "en", { numeric: true });
}

function rank(country: Country): number {
  return country.maxSeverity === undefined ? NO_RANK : SEVERITY_ORDER.indexOf(country.maxSeverity);
}

/**
 * Countries with active events first, most severe then busiest, then the rest by name. A filter
 * keeps every match, ranked like the search field's.
 */
function directory(countries: readonly Country[], filter: string): CountryDirectory {
  const trimmed = filter.trim();
  const shown = trimmed === "" ? countries : matching(trimmed, countries, Number.POSITIVE_INFINITY);
  return {
    active: shown
      .filter((country) => country.activeEvents > 0)
      .toSorted(
        (left, right) => rank(right) - rank(left) || right.activeEvents - left.activeEvents || byName(left, right),
      ),
    quiet: shown.filter((country) => country.activeEvents === 0).toSorted(byName),
  };
}

/** Active · Severe+ · Official warnings (DESIGN.md §10.2 item 2); warnings are a lower bound when capped. */
function countryCells(summary: CountrySummary): StatCell[] {
  const warnings = warningCount(summary);
  return [
    { value: String(summary.country.activeEvents), label: "Active" },
    { value: String(highSeverityCount(summary)), label: "Severe+" },
    { value: `${warnings.value}${warnings.isLowerBound ? "+" : ""}`, label: "Official warnings" },
  ];
}

/** "Wildfire · Severe": a matrix cell in words. */
function cellWords(cell: MatrixCell): string {
  return `${typeLabel(cell.type)} · ${severityLabel(cell.severity)}`;
}

/**
 * Says when the list holds fewer than the numbers above it: the server lists the 100 most severe.
 * Undefined when the list is whole.
 */
function capNote(summary: CountrySummary, listed: number, cell: MatrixCell | undefined): string | undefined {
  if (!isListCapped(summary)) {
    return undefined;
  }
  if (cell !== undefined) {
    const total = countIn(summary, cell);
    if (listed >= total) {
      return undefined;
    }
    return `Showing ${listed} of ${total}. The list holds the ${summary.events.length} most severe events in ${summary.country.name}.`;
  }
  return `Showing the ${summary.events.length} most severe of ${summary.country.activeEvents} events.`;
}

/** "1 active event", "3 active events" in or near a country, for the list's labels. */
function activeWords(country: Pick<Country, "name" | "activeEvents" | "maxSeverity">): string {
  if (country.activeEvents === 0) {
    return `${country.name}, no active events`;
  }
  const events = country.activeEvents === 1 ? "1 active event" : `${country.activeEvents} active events`;
  const worst = country.maxSeverity === undefined ? "" : `, up to ${severityLabel(country.maxSeverity).toLowerCase()}`;
  // Offshore events within 250 km count too (ADR 4), so "in or near".
  return `${country.name}, ${events} in or near it${worst}`;
}

export type { CountryDirectory };
export { activeWords, capNote, cellWords, countryCells, directory };

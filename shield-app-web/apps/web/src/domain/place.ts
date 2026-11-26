import type { HazardEvent } from "@shield/api-client";

const regionNames = new Intl.DisplayNames(["en"], { type: "region" });
/** What `Intl.DisplayNames` accepts without throwing: an ISO 3166-1 alpha-2 code. */
const ISO2 = /^[A-Z]{2}$/u;

/** The English name for an ISO 3166-1 alpha-2 code, or the code itself when there is none. */
function countryName(iso2: string): string {
  return ISO2.test(iso2) ? (regionNames.of(iso2) ?? iso2) : iso2;
}

/** "Chile", or "Off the coast of Chile, 38 km": never "in" for nearby (DESIGN.md §15). */
function place(event: Pick<HazardEvent, "countries">): string | undefined {
  const names = event.countries.map((country) => {
    const name = countryName(country.iso2);
    if (country.relation === "within") {
      return name;
    }
    return country.distanceKm === undefined
      ? `Off the coast of ${name}`
      : `Off the coast of ${name}, ${Math.round(country.distanceKm)} km`;
  });
  return names.length > 0 ? names.join(", ") : undefined;
}

/**
 * "Near Chile, 38 km offshore" when the event is offshore of `iso2`; undefined when it is within,
 * which a country's own list needn't say. Never "in" for an offshore event (ADR 4).
 */
function nearbyWords(event: Pick<HazardEvent, "countries">, iso2: string, name: string): string | undefined {
  const link = event.countries.find((country) => country.iso2 === iso2);
  if (link?.relation !== "nearby") {
    return undefined;
  }
  return link.distanceKm === undefined
    ? `Near ${name}, offshore`
    : `Near ${name}, ${Math.round(link.distanceKm)} km offshore`;
}

export { countryName, nearbyWords, place };

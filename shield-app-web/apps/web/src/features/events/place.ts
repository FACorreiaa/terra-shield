import type { HazardEvent } from "@shield/api-client";

const regionNames = new Intl.DisplayNames(["en"], { type: "region" });

/** "Chile", or "Off the coast of Chile, 38 km": never "in" for nearby (DESIGN.md §15). */
function place(event: Pick<HazardEvent, "countries">): string | undefined {
  const names = event.countries.map((country) => {
    const name = regionNames.of(country.iso2) ?? country.iso2;
    if (country.relation === "within") {
      return name;
    }
    return country.distanceKm === undefined
      ? `Off the coast of ${name}`
      : `Off the coast of ${name}, ${Math.round(country.distanceKm)} km`;
  });
  return names.length > 0 ? names.join(", ") : undefined;
}

export { place };

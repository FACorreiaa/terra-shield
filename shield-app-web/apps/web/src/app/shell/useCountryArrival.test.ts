import type { HazardEvent } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import type { Country, CountrySummary } from "@/domain/countries.ts";

import { arrivalBox } from "./useCountryArrival.ts";

const PORTUGAL: Country = { iso2: "PT", iso3: "PRT", name: "Portugal", activeEvents: 0, bbox: [-9.5, 37, -6.2, 42.2] };
// Fiji's boundary spans ±180°, so its server box is the whole band of longitudes.
const FIJI: Country = { iso2: "FJ", iso3: "FJI", name: "Fiji", activeEvents: 1, bbox: [-180, -21, 180, -12.5] };

const NOW = "2027-01-15T08:00:00Z";

/** Only the centroid matters to the box. */
function quake(longitude: number, latitude: number): HazardEvent {
  return {
    id: `${longitude},${latitude}`,
    source: "test",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity: "moderate",
    title: "e",
    centroid: { latitude, longitude },
    countries: [],
    observedAt: NOW,
    updatedAt: NOW,
    fetchedAt: NOW,
    attrs: {},
  };
}

function summary(country: Country, centroids: readonly [number, number][]): CountrySummary {
  return {
    country,
    counts: [],
    coverage: { officialWarnings: false, sources: [] },
    events: centroids.map(([longitude, latitude]) => quake(longitude, latitude)),
    recentChanges: [],
    generatedAt: NOW,
  };
}

describe(arrivalBox, () => {
  it("waits for the answer", () => {
    expect(arrivalBox({ kind: "loading" }, PORTUGAL)).toBeUndefined();
  });

  it("frames the summary's country, or the list's boundary when the summary failed", () => {
    const portugal = { west: -9.5, south: 37, east: -6.2, north: 42.2 };
    expect(
      arrivalBox({ kind: "loaded", summary: summary(PORTUGAL, []), refreshFailed: false }, undefined),
    ).toStrictEqual(portugal);
    expect(arrivalBox({ kind: "failed" }, PORTUGAL)).toStrictEqual(portugal);
    expect(arrivalBox({ kind: "notFound" }, undefined)).toBeUndefined();
  });

  it("frames a country across ±180° by its events, the short way round", () => {
    const box = arrivalBox(
      {
        kind: "loaded",
        summary: summary(FIJI, [
          [178.4, -18.1],
          [-179.8, -16.6],
        ]),
        refreshFailed: false,
      },
      FIJI,
    );
    expect(box).toStrictEqual({ west: 178.4, south: -18.1, east: -179.8, north: -16.6 });
  });
});

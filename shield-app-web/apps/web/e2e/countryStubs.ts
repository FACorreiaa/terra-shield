import type { Page } from "@playwright/test";

const MINUTE_MS = 60_000;

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * MINUTE_MS).toISOString();
}

function country(iso2: string, name: string, active?: Record<string, unknown>): Record<string, unknown> {
  return { iso2, iso3: `${iso2}X`, name, bbox: [-75.6, -56, -66.4, -17.5], ...(active ?? { activeEvents: 0 }) };
}

const COUNTRIES = [
  country("CL", "Chile", { activeEvents: 3, maxSeverity: "severe" }),
  country("PT", "Portugal"),
  country("JP", "Japan", { activeEvents: 1, maxSeverity: "moderate" }),
  country("FJ", "Fiji"),
];

function event(id: string, title: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    source: "usgs",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity: "severe",
    sourceSeverity: "M 6.1 mww",
    title,
    centroid: { latitude: -30.2, longitude: -71.6 },
    countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
    observedAt: minutesAgo(30),
    updatedAt: minutesAgo(10),
    fetchedAt: minutesAgo(1),
    attrs: {},
    ...extra,
  };
}

const QUAKE = "6f1c2a7e-0000-4000-8000-0000000000d1";
const FIRE = "6f1c2a7e-0000-4000-8000-0000000000d2";
const FLOOD = "6f1c2a7e-0000-4000-8000-0000000000d3";

const CHILE_EVENTS = [
  event(QUAKE, "M 6.1 - 38 km W of Coquimbo, Chile"),
  event(FIRE, "Wildfire near Valparaíso", {
    type: "wildfire",
    kind: "warning",
    severity: "moderate",
    sourceSeverity: "Alerta Amarilla",
    centroid: { latitude: -33, longitude: -71.5 },
    countries: [{ iso2: "CL", relation: "within" }],
  }),
  event(FLOOD, "Flooding in Biobío", {
    type: "flood",
    kind: "warning",
    severity: "moderate",
    sourceSeverity: "Alerta Amarilla",
    centroid: { latitude: -37.5, longitude: -72.4 },
    countries: [{ iso2: "CL", relation: "within" }],
  }),
];

function chileSummary(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    country: COUNTRIES[0],
    counts: [
      { type: "earthquake", severity: "severe", count: 1 },
      { type: "wildfire", severity: "moderate", count: 1 },
      { type: "flood", severity: "moderate", count: 1 },
    ],
    coverage: { officialWarnings: false, sources: ["usgs", "gdacs"] },
    events: CHILE_EVENTS,
    recentChanges: [
      {
        eventId: FIRE,
        title: "Wildfire near Valparaíso",
        at: minutesAgo(40),
        changes: { severity: { from: "minor", to: "moderate" } },
      },
    ],
    generatedAt: minutesAgo(1),
    ...extra,
  };
}

function quietSummary(): Record<string, unknown> {
  return {
    country: COUNTRIES[1],
    counts: [],
    coverage: { officialWarnings: true, sources: ["meteoalarm"] },
    events: [],
    recentChanges: [],
    generatedAt: minutesAgo(2),
  };
}

/** The map's list (empty unless given), the country list, and summaries by code (a status for a failure). */
async function stubCountries(
  page: Page,
  summaries: Readonly<Record<string, Record<string, unknown> | number>>,
  listed: readonly Record<string, unknown>[] = [],
): Promise<void> {
  await page.route("**/v1/events?**", (route) =>
    route.fulfill({ json: { events: listed, clusters: [], truncated: false, generatedAt: new Date().toISOString() } }),
  );
  await page.route("**/v1/countries", (route) => route.fulfill({ json: { countries: COUNTRIES } }));
  await page.route("**/v1/countries/*/summary", (route) => {
    const code = new URL(route.request().url()).pathname.split("/").at(-2) ?? "";
    const answer = summaries[code] ?? 404;
    return typeof answer === "number"
      ? route.fulfill({ status: answer, json: { error: "no" } })
      : route.fulfill({ json: answer });
  });
}

export { CHILE_EVENTS, chileSummary, event, FIRE, FLOOD, QUAKE, quietSummary, stubCountries };

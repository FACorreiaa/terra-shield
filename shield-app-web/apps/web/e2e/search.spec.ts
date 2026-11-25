import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";
import { openHalf } from "./sheet.ts";

const MINUTE_MS = 60_000;
/** The least zoom a fitted Chile reaches on a 1280 px screen. */
const COUNTRY_ZOOM = 3;
const QUAKE = "M 6.1 - 38 km W of Coquimbo, Chile";
const STORM = "Storm warning for Lisbon";

interface Where {
  severity: string;
  latitude: number;
  longitude: number;
}

function event(id: string, title: string, { severity, latitude, longitude }: Where): Record<string, unknown> {
  const at = new Date(Date.now() - 5 * MINUTE_MS).toISOString();
  return {
    id: `6f1c2a7e-0000-4000-8000-00000000000${id}`,
    source: "usgs",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity,
    title,
    centroid: { latitude, longitude },
    countries: [],
    observedAt: at,
    updatedAt: at,
    fetchedAt: new Date().toISOString(),
    attrs: {},
  };
}

const EVENTS = [
  event("1", QUAKE, { severity: "severe", latitude: -30.2, longitude: -71.6 }),
  event("2", STORM, { severity: "moderate", latitude: 38.7, longitude: -9.1 }),
];
const COUNTRIES = [
  {
    iso2: "CL",
    iso3: "CHL",
    name: "Chile",
    activeEvents: 1,
    maxSeverity: "severe",
    bbox: [-75.6, -55.9, -66.9, -17.5],
  },
  { iso2: "CN", iso3: "CHN", name: "China", activeEvents: 0, bbox: [73.5, 18.2, 134.8, 53.6] },
  { iso2: "PT", iso3: "PRT", name: "Portugal", activeEvents: 1, maxSeverity: "moderate", bbox: [-9.5, 37, -6.2, 42.2] },
];

/** Stubs events and countries; returns each events request's query, in order. */
async function serve(page: Page, countries: "ok" | "fail" = "ok"): Promise<URLSearchParams[]> {
  const queries: URLSearchParams[] = [];
  await page.route("**/v1/events?**", async (route) => {
    queries.push(new URL(route.request().url()).searchParams);
    await route.fulfill({
      json: { events: EVENTS, clusters: [], truncated: false, generatedAt: new Date().toISOString() },
    });
  });
  await page.route("**/v1/countries", (route) =>
    countries === "ok" ? route.fulfill({ json: { countries: COUNTRIES } }) : route.fulfill({ status: 500, json: {} }),
  );
  return queries;
}

async function seriousViolations(page: Page, selector: string): Promise<string[]> {
  const results = await new AxeBuilder({ page }).include(selector).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

/** The view's box from an events request: `west,south,east,north`. */
function boxOf(query: URLSearchParams | undefined): number[] {
  return (query?.get("bbox") ?? "").split(",").map(Number);
}

test.describe("search on a wide screen", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("/ focuses the field, which finds countries and events in view", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    await expect(page.getByRole("button", { name: new RegExp(QUAKE, "u") })).toBeVisible();
    await page.keyboard.press("/");
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await expect(field).toBeFocused();
    // On a wide screen the list stays until there is text, so Tab walks on into it (as on the Mac).
    await expect(page.getByRole("button", { name: new RegExp(QUAKE, "u") })).toBeVisible();
    await page.keyboard.press("Tab");
    await expect(page.locator("[aria-label='Sidebar'] :focus")).toHaveCount(1);

    await field.fill("chi");
    const countries = page.getByRole("region", { name: "Countries" });
    await expect(countries.getByRole("button")).toHaveText([/Chile.*1 active event/u, /China.*No active events/u]);
    const events = page.getByRole("region", { name: "Events in view" });
    await expect(events.getByRole("button", { name: new RegExp(QUAKE, "u") })).toBeVisible();
    await expect(events.getByRole("button", { name: new RegExp(STORM, "u") })).toBeHidden();
    await expect(page.getByRole("status").filter({ hasText: /results?$/u })).toHaveText("3 results");
    expect(await seriousViolations(page, "[aria-label='Sidebar']")).toStrictEqual([]);

    await field.fill("tsunami");
    await expect(page.getByText("No event in view matches.")).toBeVisible();
    await expect(page.getByRole("status").filter({ hasText: /results?$/u })).toHaveText("No results");
  });

  test("choosing a country ends the search, drops the selection and frames the country", async ({ page }) => {
    const queries = await serve(page);
    await page.goto("/map");
    await page.getByRole("button", { name: new RegExp(STORM, "u") }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.fill("chile");
    await page.getByRole("region", { name: "Countries" }).getByRole("button", { name: /Chile/u }).click();
    await expect(field).toHaveValue("");
    await expect(field).toBeFocused();
    await expect(inspector).toBeHidden();
    // The next view zooms in and holds Chile. A 1280 px globe at zoom 3 still spans ~175°, so the
    // box's width says little; the zoom says the map moved in.
    await expect
      .poll(() => {
        const last = queries.at(-1);
        const [west = 0, south = 0, east = 0, north = 0] = boxOf(last);
        return Number(last?.get("zoom")) >= COUNTRY_ZOOM && west < -71 && east > -71 && south < -30 && north > -30;
      })
      .toBe(true);
  });

  test("choosing an event with the keyboard leaves focus on its row", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.fill("lisbon");
    await page.getByRole("region", { name: "Events in view" }).getByRole("button").first().focus();
    await page.keyboard.press("Enter");
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();
    await expect(
      page.getByRole("complementary", { name: "Sidebar" }).getByRole("button", { name: new RegExp(STORM, "u") }),
    ).toBeFocused();
  });

  test("a country list that fails leaves events searchable, with no error", async ({ page }) => {
    await serve(page, "fail");
    await page.goto("/map");
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.fill("coquimbo");
    await expect(page.getByRole("button", { name: new RegExp(QUAKE, "u") })).toBeVisible();
    await expect(page.getByRole("region", { name: "Countries" })).toBeHidden();
    await expect(page.getByText(/went wrong|didn’t load|isn’t responding/u)).toBeHidden();
  });

  test("Escape ends the search first, then clears the selection", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    await page.getByRole("button", { name: new RegExp(STORM, "u") }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();

    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.fill("coquimbo");
    // From a result, not the field: the page's own Escape.
    await page.getByRole("region", { name: "Events in view" }).getByRole("button").first().focus();
    await page.keyboard.press("ArrowDown");
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(field).toHaveValue("");
    await expect(field).toBeFocused();
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();

    // Inside the field, Escape ends only the search.
    await field.fill("lisbon");
    await field.press("Escape");
    await expect(field).toHaveValue("");
    await expect(inspector.getByRole("heading", { name: STORM })).toBeVisible();

    await field.blur();
    await page.keyboard.press("Escape");
    await expect(inspector).toBeHidden();
  });
});

test.describe("search on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("focusing opens the sheet to full height, Clear keeps it there, and Cancel puts it back", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.tap();
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");
    await field.fill("chi");
    await expect(page.getByRole("region", { name: "Countries" })).toBeVisible();
    expect(await seriousViolations(page, "[aria-label='Events']")).toStrictEqual([]);

    await page.getByRole("button", { name: "Clear search" }).tap();
    await expect(field).toHaveValue("");
    await expect(field).toBeFocused();
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");

    await page.getByRole("button", { name: "Cancel" }).tap();
    await expect(field).not.toBeFocused();
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
  });

  test("an event chosen from collapsed shows at half height", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.tap();
    await field.fill("coquimbo");
    await page
      .getByRole("region", { name: "Events in view" })
      .getByRole("button", { name: new RegExp(QUAKE, "u") })
      .tap();
    await expect(page.getByRole("heading", { name: QUAKE })).toBeVisible();
    await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
  });

  test("searches over a selection, and a chosen event shows at half height", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    await openHalf(page);
    await page.getByRole("button", { name: new RegExp(QUAKE, "u") }).tap();
    await expect(page.getByRole("heading", { name: QUAKE })).toBeVisible();

    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.tap();
    await field.fill("lisbon");
    await page
      .getByRole("region", { name: "Events in view" })
      .getByRole("button", { name: new RegExp(STORM, "u") })
      .tap();
    await expect(page.getByRole("heading", { name: STORM })).toBeVisible();
    await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
  });

  test("choosing a country collapses the sheet, then frames the country", async ({ page }) => {
    const queries = await serve(page);
    await page.goto("/map");
    await expect.poll(() => queries.length).toBeGreaterThan(0);
    const startZoom = Number(queries[0]?.get("zoom"));
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.tap();
    await field.fill("chile");
    await page.getByRole("region", { name: "Countries" }).getByRole("button", { name: /Chile/u }).tap();
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
    await expect
      .poll(() => {
        const last = queries.at(-1);
        const [west = 0, south = 0, east = 0, north = 0] = boxOf(last);
        return Number(last?.get("zoom")) > startZoom && west < -71 && east > -71 && south < -30 && north > -30;
      })
      .toBe(true);
  });
});

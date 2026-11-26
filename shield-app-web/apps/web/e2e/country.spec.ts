import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { chileSummary, event, QUAKE, quietSummary, stubCountries } from "./countryStubs.ts";
import { expect, test } from "./fixtures.ts";

/** Serious and critical axe findings on the page. */
async function violations(page: Page): Promise<string[]> {
  const results = await new AxeBuilder({ page }).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

test.describe("countries, desktop", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("lists countries, active first, filters them, and opens one", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/map");
    await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
    await page.keyboard.press("2");
    await expect(page).toHaveURL(/\/countries$/u);
    const sidebar = page.getByRole("complementary", { name: "Sidebar" });
    // One field here: the list's own filter, not the map's search.
    await expect(sidebar.getByRole("searchbox", { name: "Search countries or events" })).toHaveCount(0);
    const active = sidebar.getByRole("region", { name: "Active events", exact: true });
    await expect(active.getByRole("link")).toHaveText([/^Chile/u, /^Japan/u]);
    await expect(
      active.getByRole("link", { name: "Chile, 3 active events in or near it, up to severe" }),
    ).toBeVisible();
    expect(await violations(page)).toStrictEqual([]);

    await sidebar.getByRole("searchbox", { name: "Filter countries" }).fill("port");
    await expect(sidebar.getByRole("link", { name: /active events?/u })).toHaveAccessibleName(
      "Portugal, no active events",
    );
    await sidebar.getByRole("searchbox", { name: "Filter countries" }).fill("");

    await active.getByRole("link", { name: /^Chile/u }).click();
    await expect(page).toHaveURL(/\/countries\/CL$/u);
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeFocused();
    await expect(sidebar.getByRole("link", { name: "Countries" })).toHaveAttribute("aria-current", "page");
  });

  test("shows a country's numbers, matrix, coverage, changes and events", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/countries/CL");
    const overview = page.getByRole("article", { name: "Chile" });
    await expect(overview.getByText(/^Updated \d+ (?:s|min) ago$/u)).toBeVisible();
    await expect(overview.getByText(/Limited coverage: no official warning service for Chile/u)).toBeVisible();
    await expect(overview.getByText("Absence of data is not safety.")).toHaveCount(1);
    await expect(overview.getByText("Raised to Moderate · 40 min ago")).toBeVisible();
    await expect(overview.getByText("Near Chile, 38 km offshore")).toBeVisible();
    expect(await violations(page)).toStrictEqual([]);

    const events = overview.getByRole("region", { name: "Events" });
    await expect(events.getByRole("listitem")).toHaveCount(3);
    // An event inside the country needs no place; only offshore ones say where.
    await expect(events.getByRole("button", { name: /Valparaíso/u })).not.toContainText("Chile");
    const cell = overview.getByRole("button", { name: "Wildfire, Moderate: 1" });
    await cell.click();
    await expect(cell).toHaveAttribute("aria-pressed", "true");
    await expect(events.getByRole("listitem")).toHaveCount(1);
    await events.getByRole("button", { name: "Remove filter: Wildfire, Moderate" }).click();
    await expect(events.getByRole("listitem")).toHaveCount(3);
  });

  test("opens a country's event beside it; Escape closes the event, then the country", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/countries/CL");
    await page
      .getByRole("region", { name: "Events" })
      .getByRole("button", { name: /Coquimbo/u })
      .click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    // The country's copy shows at once, though the map's list doesn't hold it.
    await expect(inspector.getByRole("heading", { name: /Coquimbo/u })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/countries/CL\\?event=${QUAKE}`, "u"));

    await page.keyboard.press("Escape");
    await expect(inspector).toHaveCount(0);
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/countries$/u);
    // Keyboard focus lands on the country's row, not lost to the page.
    await expect(page.getByRole("link", { name: /^Chile, 3 active events/u })).toBeFocused();
  });

  test("keeps a country's filter through a search, and frames it again when chosen again", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/countries/CL");
    const cell = page.getByRole("button", { name: "Wildfire, Moderate: 1" });
    await cell.click();
    const field = page.getByRole("searchbox", { name: "Search countries or events" });
    await field.fill("chi");
    await page.keyboard.press("Escape");
    await expect(cell).toHaveAttribute("aria-pressed", "true");
    await expect(field).toBeFocused();

    await field.fill("chile");
    await page.getByRole("region", { name: "Countries" }).getByRole("button", { name: /Chile/u }).click();
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeFocused();
  });

  test("ends a search carried into Countries", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/map");
    await page.getByRole("searchbox", { name: "Search countries or events" }).fill("ch");
    await page.getByRole("navigation", { name: "Sections" }).getByRole("link", { name: "Countries" }).click();
    await page.getByRole("link", { name: /^Chile, 3 active events/u }).click();
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeVisible();
    await expect(page.getByRole("searchbox", { name: "Search countries or events" })).toHaveValue("");
  });

  test("leaves ↑/↓ to the Monitor list, and keeps an event in a shared country link", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() }, [event(QUAKE, "M 6.1 - 38 km W of Coquimbo, Chile")]);
    await page.goto("/countries/CL");
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(page).toHaveURL(/\/countries\/CL$/u);

    await page.goto(`/c/cl?event=${QUAKE}`);
    await expect(page).toHaveURL(new RegExp(`/countries/cl\\?event=${QUAKE}$`, "u"));
  });

  test("an event's country opens its overview and closes the event", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() }, [event(QUAKE, "M 6.1 - 38 km W of Coquimbo, Chile")]);
    await page.goto(`/map?event=${QUAKE}`);
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await inspector.getByRole("link", { name: "Off the coast of Chile, 38 km" }).click();
    await expect(page).toHaveURL(/\/countries\/CL$/u);
    await expect(inspector).toHaveCount(0);
  });

  test("asks for the countries again on each visit", async ({ page }) => {
    await stubCountries(page, {});
    let asked = 0;
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === "/v1/countries") {
        asked += 1;
      }
    });
    await page.goto("/countries");
    await expect(page.getByRole("link", { name: /^Chile, 3 active events/u })).toBeVisible();
    const before = asked;
    await page.getByRole("link", { name: "Monitor" }).click();
    await expect(page.getByRole("searchbox", { name: "Filter countries" })).toHaveCount(0);
    await page.getByRole("navigation", { name: "Sections" }).getByRole("link", { name: "Countries" }).click();
    await expect.poll(() => asked).toBeGreaterThan(before);
  });

  test("says when there is no boundary, and when a quiet country has nothing", async ({ page }) => {
    await stubCountries(page, { PT: quietSummary() });
    await page.goto("/c/zz");
    await expect(page).toHaveURL(/\/countries\/zz$/u);
    await expect(page.getByText("We have no boundary for this place.")).toBeVisible();
    await page.goto("/countries/123");
    await expect(page.getByText("We have no boundary for this place.")).toBeVisible();

    await page.goto("/countries/PT");
    await expect(page.getByText("No active events in Portugal from covered sources.")).toBeVisible();
    await expect(page.getByText("Official warnings cover Portugal.")).toBeVisible();
  });

  test("offers Retry when a country can't load", async ({ page }) => {
    let fail = true;
    await stubCountries(page, {});
    await page.route("**/v1/countries/CL/summary", (route) =>
      fail ? route.fulfill({ status: 503, json: {} }) : route.fulfill({ json: chileSummary() }),
    );
    await page.goto("/countries/CL");
    await expect(page.getByText(/The country couldn’t be loaded/u)).toBeVisible();
    fail = false;
    await page.getByRole("button", { name: "Retry" }).click();
    await expect(page.getByText(/^Updated \d+ (?:s|min) ago$/u)).toBeVisible();
  });
});

test.describe("countries, phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("opens a country at half height, with its own numbers only", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/countries/CL");
    await expect(page.getByRole("button", { name: /^Resize sheet/u })).toHaveAccessibleName(
      "Resize sheet, now half height",
    );
    await expect(page.getByRole("heading", { name: /^Chile/u })).toBeInViewport();
    // The map's full strip steps aside: the country has its own numbers.
    await expect(page.getByText("High severity", { exact: true })).toBeHidden();
    await expect(page.getByRole("article", { name: "Chile" }).getByText("Severe+")).toBeVisible();
    expect(await violations(page)).toStrictEqual([]);
  });

  test("keeps the country's filter through an event opened from it", async ({ page }) => {
    await stubCountries(page, { CL: chileSummary() });
    await page.goto("/countries/CL");
    const cell = page.getByRole("button", { name: "Earthquake, Severe: 1" });
    await cell.tap();
    await page
      .getByRole("region", { name: "Events" })
      .getByRole("button", { name: /Coquimbo/u })
      .tap();
    await expect(page.getByRole("heading", { name: /Coquimbo/u })).toBeVisible();
    await page.getByRole("button", { name: "Close", exact: true }).tap();
    await expect(cell).toHaveAttribute("aria-pressed", "true");
  });

  test("lists countries with one field, its own", async ({ page }) => {
    await stubCountries(page, {});
    await page.goto("/countries");
    await expect(page.getByRole("searchbox", { name: "Filter countries" })).toBeVisible();
    await expect(page.getByRole("searchbox", { name: "Search countries or events" })).toHaveCount(0);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Countries");
  });
});

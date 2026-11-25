import { AxeBuilder } from "@axe-core/playwright";
import type { Page, Route } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const MINUTE_MS = 60_000;
const TITLE = "M 6.1 - 38 km W of Coquimbo, Chile";

function list(events: readonly Record<string, unknown>[]): Record<string, unknown> {
  return { events, clusters: [], truncated: false, generatedAt: new Date().toISOString() };
}

const quake = {
  id: "6f1c2a7e-0000-4000-8000-000000000001",
  source: "usgs",
  type: "earthquake",
  kind: "observed",
  status: "active",
  severity: "severe",
  sourceSeverity: "M 6.1 mww",
  title: TITLE,
  centroid: { latitude: -30.2, longitude: -71.6 },
  countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
  observedAt: new Date(Date.now() - 5 * MINUTE_MS).toISOString(),
  updatedAt: new Date(Date.now() - 5 * MINUTE_MS).toISOString(),
  fetchedAt: new Date().toISOString(),
  attrs: {},
};

/** Serves `/v1/events` from `respond`, and records each request's query. */
async function serve(page: Page, respond: (route: Route) => Promise<void>): Promise<URLSearchParams[]> {
  const queries: URLSearchParams[] = [];
  await page.route("**/v1/events?**", async (route) => {
    queries.push(new URL(route.request().url()).searchParams);
    await respond(route);
  });
  return queries;
}

async function seriousViolations(page: Page, selector: string): Promise<string[]> {
  const results = await new AxeBuilder({ page }).include(selector).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

test.describe("monitor states", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("an empty view says what isn't covered, and blames the filters when they are on", async ({ page }) => {
    const queries = await serve(page, (route) => route.fulfill({ json: list([]) }));
    await page.goto("/map");
    await expect(page.getByText("No active events in this view from covered sources.")).toBeVisible();
    await expect(page.getByText(/Absence of data is not safety/u)).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Live events" }).getByRole("button", { name: "Zoom out" }),
    ).toBeVisible();

    const strip = page.getByRole("button", { name: /High severity/u });
    await expect(strip).toHaveAttribute("aria-pressed", "false");
    await strip.click();
    await expect(strip).toHaveAttribute("aria-pressed", "true");
    await expect.poll(() => queries.at(-1)?.get("severity")).toBe("extreme,severe");
    await expect(page.getByText("No events in this view match your filters.")).toBeVisible();
    await page.getByRole("button", { name: "Reset filters" }).click();
    await expect(page.getByText("No active events in this view from covered sources.")).toBeVisible();
  });

  test("keeps the events on screen when a later load fails, with a banner and Retry", async ({ page }) => {
    const answers = { fail: false };
    await serve(page, (route) =>
      answers.fail
        ? route.fulfill({ status: 503, json: { error: true, reason: "database down" } })
        : route.fulfill({ json: list([quake]) }),
    );
    await page.goto("/map");
    await expect(page.getByRole("button", { name: new RegExp(TITLE, "u") })).toBeVisible();

    answers.fail = true;
    await page.getByRole("button", { name: "Zoom in" }).click();
    await expect(page.getByRole("alert")).toContainText("Something went wrong loading events.");
    await expect(page.getByRole("alert")).not.toContainText("database down");
    await expect(page.getByRole("button", { name: new RegExp(TITLE, "u") })).toBeVisible();

    answers.fail = false;
    await page.getByRole("alert").getByRole("button", { name: "Retry" }).click();
    await expect(page.getByRole("alert")).toBeHidden();
  });

  test("says the server isn't responding when nothing answers", async ({ page }) => {
    await serve(page, (route) => route.abort());
    await page.goto("/map");
    await expect(page.getByText("Shield’s server isn’t responding.").first()).toBeVisible({ timeout: 10_000 });
  });

  test("steps through the list in the order it is shown", async ({ page }) => {
    const minor = {
      ...quake,
      id: "6f1c2a7e-0000-4000-8000-000000000002",
      severity: "minor",
      title: "M 3.0 - Minor quake",
    };
    const extreme = {
      ...quake,
      id: "6f1c2a7e-0000-4000-8000-000000000003",
      severity: "extreme",
      title: "M 7.4 - Extreme quake",
    };
    await serve(page, (route) => route.fulfill({ json: list([minor, quake, extreme]) }));
    await page.goto("/map");
    await expect(page.getByRole("button", { name: /M 7\.4/u })).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("heading", { name: "M 7.4 - Extreme quake" })).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("heading", { name: TITLE })).toBeVisible();
  });

  test("says when the data is from while offline, and keeps it", async ({ page, context }) => {
    await serve(page, (route) => route.fulfill({ json: list([quake]) }));
    await page.goto("/map");
    await expect(page.getByText("Updated now")).toBeVisible();
    await context.setOffline(true);
    await expect(page.getByText(/^Offline · showing data from \d\d:\d\d UTC$/u)).toBeVisible();
    await expect(page.getByRole("button", { name: new RegExp(TITLE, "u") })).toBeVisible();
    await context.setOffline(false);
    await expect(page.getByText("Updated now")).toBeVisible();
  });

  test("turns the capsule to a warning after three missed refreshes", async ({ page }) => {
    await page.clock.install();
    const answers = { fail: false };
    await serve(page, (route) =>
      answers.fail ? route.fulfill({ status: 503, json: { error: true } }) : route.fulfill({ json: list([quake]) }),
    );
    await page.goto("/map");
    await expect(page.getByText("Updated now")).toBeVisible();
    answers.fail = true;
    await page.clock.fastForward("04:00");
    await expect(page.getByText("Last update 4 min ago")).toBeVisible();
  });
});

test.describe("filters", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("the sheet filters the map, shows a removable chip, and survives a reload", async ({ page }) => {
    const queries = await serve(page, (route) => route.fulfill({ json: list([quake]) }));
    await page.goto("/map");
    await page.getByRole("button", { name: "Filters" }).click();
    const sheet = page.getByRole("dialog", { name: "Filters" });
    await expect(sheet).toBeVisible();
    expect(await seriousViolations(page, "dialog")).toStrictEqual([]);

    await sheet.getByRole("radio", { name: "Severe and above" }).check({ force: true });
    await sheet.getByRole("button", { name: "Done" }).click();
    await expect(page.getByRole("button", { name: "Remove filter: Severe+" })).toBeVisible();
    await expect.poll(() => queries.at(-1)?.get("severity")).toBe("extreme,severe");

    await page.reload();
    await expect(page.getByRole("button", { name: "Remove filter: Severe+" })).toBeVisible();
    await page.getByRole("button", { name: "Remove filter: Severe+" }).click();
    await expect(page.getByRole("button", { name: "Remove filter: Severe+" })).toBeHidden();
    await expect.poll(() => queries.at(-1)?.get("severity")).toBeNull();
  });

  test("the last kind can't be turned off", async ({ page }) => {
    await serve(page, (route) => route.fulfill({ json: list([quake]) }));
    await page.goto("/map");
    await page.getByRole("button", { name: "Filters" }).click();
    const sheet = page.getByRole("dialog", { name: "Filters" });
    await sheet.getByRole("switch", { name: "Observed" }).uncheck({ force: true });
    await sheet.getByRole("switch", { name: "Forecast" }).uncheck({ force: true });
    await expect(sheet.getByRole("switch", { name: "Official warning" })).toBeDisabled();
    await expect(sheet.getByRole("button", { name: "Reset" })).toBeVisible();
  });
});

test.describe("phone filters", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("opens as a sheet with no accessibility violations", async ({ page }) => {
    await serve(page, (route) => route.fulfill({ json: list([quake]) }));
    await page.goto("/map");
    await expect(page.getByText(/1\s+event/u)).toBeVisible();
    await page.getByRole("button", { name: "Filters" }).tap();
    await expect(page.getByRole("dialog", { name: "Filters" })).toBeVisible();
    expect(await seriousViolations(page, "dialog")).toStrictEqual([]);
  });

  test("shows the full strip from half height, and a failure without losing the events", async ({ page }) => {
    const answers = { fail: false };
    await serve(page, (route) =>
      answers.fail ? route.fulfill({ status: 503, json: { error: true } }) : route.fulfill({ json: list([quake]) }),
    );
    await page.goto("/map");
    await expect(page.getByText(/1\s+event/u)).toBeVisible();
    await page.getByRole("button", { name: /Resize sheet/u }).tap();
    await expect(page.getByRole("button", { name: /High severity/u })).toBeVisible();

    answers.fail = true;
    await page.getByRole("button", { name: "Zoom in" }).tap();
    await expect(page.getByRole("alert")).toContainText("Something went wrong loading events.");
    await expect(page.getByRole("button", { name: new RegExp(TITLE, "u") })).toBeVisible();
  });
});

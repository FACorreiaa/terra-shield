import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const MINUTE_MS = 60_000;
const LISTED = "6f1c2a7e-0000-4000-8000-0000000000a1";
const UNLISTED = "6f1c2a7e-0000-4000-8000-0000000000b2";
const TITLE = "M 6.1 - 38 km W of Coquimbo, Chile";
const EVENT_PAGE = "https://earthquake.usgs.gov/earthquakes/eventpage/us7000test";

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * MINUTE_MS).toISOString();
}

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
    url: EVENT_PAGE,
    centroid: { latitude: -30.2, longitude: -71.6 },
    countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
    observedAt: minutesAgo(30),
    updatedAt: minutesAgo(10),
    fetchedAt: minutesAgo(1),
    attrs: { magnitude: 6.1, depthKm: 35 },
    ...extra,
  };
}

function source(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: "usgs",
    name: "USGS Earthquake Hazards Program",
    url: "https://earthquake.usgs.gov",
    license: "Public domain",
    attribution: "U.S. Geological Survey",
    pollIntervalSeconds: 60,
    enabled: true,
    lastSuccessAt: minutesAgo(1),
    ...extra,
  };
}

function detail(id: string, title: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    ...event(id, title, extra),
    geometry: { type: "Point", coordinates: [-71.6, -30.2] },
    sourceInfo: source(),
    revisions: [{ at: minutesAgo(10), changes: { severity: { from: "moderate", to: "severe" } } }],
  };
}

interface Stubs {
  listed?: readonly Record<string, unknown>[];
  details?: Readonly<Record<string, Record<string, unknown> | number>>;
  sources?: readonly Record<string, unknown>[];
}

/** Routes the list, each event's detail (a body, or a status for a failure) and the sources. */
async function stub(
  page: Page,
  { listed = [event(LISTED, TITLE)], details = {}, sources = [source()] }: Stubs,
): Promise<void> {
  await page.route("**/v1/events?**", (route) =>
    route.fulfill({ json: { events: listed, clusters: [], truncated: false, generatedAt: new Date().toISOString() } }),
  );
  await page.route("**/v1/sources", (route) => route.fulfill({ json: { sources } }));
  await page.route("**/v1/events/*", (route) => {
    const id = new URL(route.request().url()).pathname.split("/").pop() ?? "";
    const answer = details[id] ?? 404;
    if (typeof answer === "number") {
      return route.fulfill({ status: answer, json: { error: "not found" } });
    }
    return route.fulfill({ json: answer });
  });
}

/** Serious and critical axe findings on the page. */
async function violations(page: Page): Promise<string[]> {
  const results = await new AxeBuilder({ page }).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

test.describe("event detail, desktop", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("shows the event in full, and Back closes it", async ({ page }) => {
    await stub(page, { details: { [LISTED]: detail(LISTED, TITLE) } });
    await page.goto("/map");
    const row = page.getByRole("button", { name: /Coquimbo/u });
    await row.click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });

    await expect(page).toHaveURL(new RegExp(`event=${LISTED}`, "u"));
    // The list stays put under a selection: the row keeps focus.
    await expect(row).toBeFocused();
    await expect(inspector.getByRole("heading", { name: TITLE })).toBeVisible();
    await expect(inspector.getByText("M 6.1 mww")).toBeVisible();
    await expect(inspector.getByText("Severity Moderate → Severe")).toBeVisible();
    await expect(inspector.getByText("USGS Earthquake Hazards Program")).toBeVisible();
    await expect(inspector.getByRole("link", { name: /Open official source/u })).toHaveAttribute("href", EVENT_PAGE);
    expect(await violations(page)).toStrictEqual([]);

    await page.goBack();
    await expect(inspector).toHaveCount(0);
    await expect(page).not.toHaveURL(/event=/u);
  });

  test("opens a shared link to an event outside the view", async ({ page }) => {
    await stub(page, { listed: [], details: { [UNLISTED]: detail(UNLISTED, "M 5.4 - Fiji region") } });
    await page.goto(`/e/${UNLISTED}`);
    await expect(page).toHaveURL(new RegExp(`/map\\?event=${UNLISTED}`, "u"));
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: "M 5.4 - Fiji region" })).toBeVisible();
  });

  test("says a dropped event is gone, and points to the source's home", async ({ page }) => {
    await stub(page, { details: { [LISTED]: 404 } });
    await page.goto("/map");
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(
      inspector.getByText(/no longer published by its source\. Open official source goes to USGS/u),
    ).toBeVisible();
    await expect(inspector.getByText("No history: the source no longer publishes this event.")).toBeVisible();
    await expect(inspector.getByRole("link", { name: /Open official source/u })).toHaveAttribute(
      "href",
      "https://earthquake.usgs.gov",
    );
  });

  test("says a link to an unknown event is gone", async ({ page }) => {
    await stub(page, { listed: [] });
    await page.goto(`/e/${UNLISTED}`);
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByText("This event is no longer published by its source.")).toBeVisible();
    await inspector.getByRole("button", { name: "Close" }).click();
    await expect(inspector).toHaveCount(0);
  });

  test("marks a stale source and a resolved event", async ({ page }) => {
    const resolved = event(UNLISTED, "M 5.0 - Offshore Chile", {
      status: "resolved",
      endedAt: minutesAgo(90),
      centroid: { latitude: -31, longitude: -72.5 },
    });
    await stub(page, {
      listed: [event(LISTED, TITLE), resolved],
      details: { [LISTED]: detail(LISTED, TITLE) },
      sources: [source({ stale: true, lastSuccessAt: minutesAgo(45) })],
    });
    await page.goto("/map");
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    await expect(inspector.getByText(/^Stale · this source last updated 45 min ago/u)).toBeVisible();

    await page.getByRole("button", { name: /Offshore Chile/u }).click();
    await expect(inspector.getByText(/^Resolved · ended/u)).toBeVisible();
    await expect(inspector.getByText(/^Stale/u)).toHaveCount(0);
  });

  test("copies the event's link", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await stub(page, { details: { [LISTED]: detail(LISTED, TITLE) } });
    await page.goto(`/map?event=${LISTED}`);
    await page.getByRole("button", { name: "Share" }).click();
    await expect(page.getByText("Link copied")).toBeVisible();
    const copied = await page.evaluate(() => navigator.clipboard.readText());
    expect(copied).toBe(`${new URL(page.url()).origin}/e/${LISTED}`);
  });
  test("switching events replaces the entry, and Back closes in one step", async ({ page }) => {
    const other = event(UNLISTED, "M 5.2 - Off Valparaiso", { centroid: { latitude: -33, longitude: -72 } });
    await stub(page, { listed: [event(LISTED, TITLE), other], details: { [LISTED]: detail(LISTED, TITLE) } });
    await page.goto("/map");
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    await page.getByRole("button", { name: /Valparaiso/u }).click();
    await expect(page).toHaveURL(new RegExp(`event=${UNLISTED}`, "u"));
    // B's detail never shows A's history.
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByText("Severity Moderate → Severe")).toHaveCount(0);

    await page.keyboard.press("1");
    await expect(page).toHaveURL(new RegExp(`event=${UNLISTED}`, "u"));
    await page.goBack();
    await expect(page).not.toHaveURL(/event=/u);
  });

  test("offers to try again when the detail can't load", async ({ page }) => {
    let fail = true;
    await stub(page, { listed: [] });
    await page.route("**/v1/events/*", (route) =>
      fail
        ? route.fulfill({ status: 503, json: {} })
        : route.fulfill({ json: detail(UNLISTED, "M 5.4 - Fiji region") }),
    );
    await page.goto(`/e/${UNLISTED}`);
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByText(/couldn’t be loaded\. Check your connection and try again\./u)).toBeVisible();
    fail = false;
    await inspector.getByRole("button", { name: "Try again" }).click();
    await expect(inspector.getByRole("heading", { name: "M 5.4 - Fiji region" })).toBeVisible();
  });
});

test.describe("event detail, phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("opens a shared link at half height", async ({ page }) => {
    await stub(page, { details: { [LISTED]: detail(LISTED, TITLE) } });
    await page.goto(`/e/${LISTED}`);
    await expect(page.getByRole("button", { name: /^Resize sheet/u })).toHaveAccessibleName(
      "Resize sheet, now half height",
    );
    await expect(page.getByRole("heading", { name: TITLE })).toBeInViewport();
    expect(await violations(page)).toStrictEqual([]);
  });

  test("moves focus into the detail and back to the row on Close", async ({ page }) => {
    await stub(page, { details: { [LISTED]: detail(LISTED, TITLE) } });
    await page.goto("/map");
    await page.getByRole("button", { name: /^Resize sheet/u }).press("Enter");
    const row = page.getByRole("button", { name: /Coquimbo/u });
    await row.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: TITLE })).toBeFocused();

    await page.getByRole("button", { name: "Close", exact: true }).press("Enter");
    await expect(page.getByRole("button", { name: /Coquimbo/u })).toBeFocused();
  });
});

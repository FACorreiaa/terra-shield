import type { Locator, Page, Request } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const EMPTY_EVENTS = { events: [], clusters: [], truncated: false, generatedAt: new Date().toISOString() };
const HOURS_AGO = 3;

function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString();
}

/** A grid answer: two cells, the newest detected 3 h ago. */
const GRID = {
  type: "FeatureCollection",
  aggregation: "grid",
  cellDegrees: 2,
  truncated: false,
  features: [
    {
      type: "Feature",
      geometry: { type: "Point", coordinates: [20, 0] },
      properties: { count: 120, maxFrp: 80, latestAt: hoursAgo(HOURS_AGO) },
    },
    {
      type: "Feature",
      geometry: { type: "Point", coordinates: [24, 2] },
      properties: { count: 2, latestAt: hoursAgo(5) },
    },
  ],
};

/** A point answer that was cut off. */
const POINTS = {
  type: "FeatureCollection",
  aggregation: "none",
  truncated: true,
  features: [
    {
      type: "Feature",
      geometry: { type: "Point", coordinates: [20, 0] },
      properties: { satellite: "NOAA-20", acquiredAt: hoursAgo(1), frp: 12, confidence: "nominal" },
    },
  ],
};

const heatButton = (page: Page): Locator => page.getByRole("button", { name: "Satellite heat detections" });
const legend = (page: Page): Locator => page.locator("[data-detection-legend] > span:not([aria-hidden])");

/** Records each `/v1/fires` request and answers with `body` (or the status given). */
async function serveFires(page: Page, body: unknown, status = 200): Promise<Request[]> {
  const requests: Request[] = [];
  await page.route("**/v1/fires?**", (route) => {
    requests.push(route.request());
    return route.fulfill({ status, json: body });
  });
  return requests;
}

test.beforeEach(async ({ page }) => {
  await page.route("**/v1/events?**", (route) => route.fulfill({ json: EMPTY_EVENTS }));
});

test.describe("satellite heat detections", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("are off by default; the control turns them on, and the choice is kept", async ({ page }) => {
    const requests = await serveFires(page, GRID);
    await page.goto("/map");
    await expect(heatButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
    expect(requests).toHaveLength(0);
    await expect(legend(page)).toHaveCount(0);

    await heatButton(page).click();
    await expect(heatButton(page)).toHaveAttribute("aria-pressed", "true");
    await expect(legend(page)).toHaveText(`Satellite heat detections · Detected ${HOURS_AGO} h ago`);
    const url = new URL(requests[0]?.url() ?? "");
    expect(url.searchParams.get("zoom")).toMatch(/^\d+$/u);
    expect(url.searchParams.get("bbox")).toMatch(/^-?[\d.]+,-?[\d.]+,-?[\d.]+,-?[\d.]+$/u);
    // No `since`: the filters' time window is for events.
    expect(url.searchParams.has("since")).toBe(false);

    await page.reload();
    await expect(heatButton(page)).toHaveAttribute("aria-pressed", "true");
    await expect(legend(page)).toBeVisible();
  });

  test("h, the filter sheet and the control are one switch; Reset turns it off", async ({ page }) => {
    await serveFires(page, GRID);
    await page.goto("/map");
    await expect(heatButton(page)).toBeVisible();
    await page.locator("body").press("h");
    await expect(heatButton(page)).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: /^Filters/u }).click();
    const sheet = page.getByRole("dialog", { name: "Filters" });
    const row = sheet.getByRole("switch", { name: "Satellite heat detection" });
    await expect(row).toBeChecked();
    await sheet.getByRole("button", { name: "Reset" }).click();
    await expect(row).not.toBeChecked();
    await sheet.getByRole("button", { name: "Done" }).click();
    await expect(heatButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(legend(page)).toHaveCount(0);
  });

  test("turned off, it asks for nothing more, even on refresh", async ({ page }) => {
    const requests = await serveFires(page, GRID);
    await page.goto("/map");
    await heatButton(page).click();
    await expect(legend(page)).toBeVisible();
    await heatButton(page).click();
    await expect(legend(page)).toHaveCount(0);
    const before = requests.length;
    await page.locator("body").press("r");
    await page.waitForTimeout(500);
    expect(requests).toHaveLength(before);
  });

  test("a failed load is announced once, for screen readers", async ({ page }) => {
    await serveFires(page, { error: "unavailable" }, 503);
    await page.goto("/map");
    await heatButton(page).click();
    await expect(page.locator("[data-detection-legend] output")).toHaveText("Heat detections didn’t load");
  });

  test("a cut-off answer asks to zoom in", async ({ page }) => {
    await serveFires(page, POINTS);
    await page.goto("/map");
    await heatButton(page).click();
    await expect(legend(page)).toHaveText("Satellite heat detections · Detected 1 h ago · Zoom in for all");
  });

  test("a failed load says so in the legend, not in the events' banner", async ({ page }) => {
    await serveFires(page, { error: "unavailable" }, 503);
    await page.goto("/map");
    await heatButton(page).click();
    await expect(legend(page)).toHaveText("Heat detections didn’t load");
    await expect(page.getByText("Something went wrong loading events.")).toHaveCount(0);
  });
});

test.describe("satellite heat detections, phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("the legend wraps rather than cut its words, clear of the controls", async ({ page }) => {
    await serveFires(page, POINTS);
    await page.goto("/map");
    await heatButton(page).click();
    await expect(legend(page)).toHaveText("Satellite heat detections · Detected 1 h ago · Zoom in for all");
    const key = await page.locator("[data-detection-legend]").boundingBox();
    const controls = await heatButton(page).boundingBox();
    expect((key?.x ?? 0) + (key?.width ?? 0)).toBeLessThanOrEqual(controls?.x ?? 0);
    // Nothing clipped: the text fits its box.
    expect(await legend(page).evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  });
});

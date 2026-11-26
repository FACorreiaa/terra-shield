import type { Locator, Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

interface Served {
  /** Each `/v1/aircraft` request's query. */
  aircraft: URLSearchParams[];
  /** Each events request's zoom: one per settled view. */
  zooms: number[];
}

/** The middle of a request's box, so an aircraft put there is in the middle of the view. */
function middle(query: URLSearchParams): { lat: number; lon: number } {
  const [west = 0, south = 0, east = 0, north = 0] = (query.get("bbox") ?? "").split(",").map(Number);
  return { lat: (south + north) / 2, lon: (west + east) / 2 };
}

/** Two aircraft: an airliner on a track in the middle of the view, and one with no track beside it. */
function answer(query: URLSearchParams): Record<string, unknown> {
  const { lat, lon } = middle(query);
  const now = new Date().toISOString();
  return {
    aircraft: [
      {
        hex: "49530a",
        callsign: "TAP215",
        registration: "CS-TXK",
        typeCode: "A21N",
        lat,
        lon,
        altitudeFt: 38_000,
        onGround: false,
        groundSpeedKt: 417,
        trackDeg: 90,
        military: false,
        positionAt: now,
      },
      { hex: "~2a0001", lat, lon: lon + 0.5, onGround: true, military: false, positionAt: now },
    ],
    dataAt: now,
    complete: true,
    attribution: "© adsb.lol contributors",
    license: "ODbL 1.0",
    licenseUrl: "https://opendatacommons.org/licenses/odbl/1-0/",
  };
}

/** No hazards (nothing over the aircraft), and the flights feed answering with `status`. */
async function serve(page: Page, status = 200): Promise<Served> {
  const served: Served = { aircraft: [], zooms: [] };
  await page.route("**/v1/events?**", (route) => {
    const query = new URL(route.request().url()).searchParams;
    served.zooms.push(Number(query.get("zoom")));
    return route.fulfill({
      json: { events: [], clusters: [], truncated: false, generatedAt: new Date().toISOString() },
    });
  });
  await page.route("**/v1/aircraft?**", (route) => {
    const query = new URL(route.request().url()).searchParams;
    served.aircraft.push(query);
    return status === 200
      ? route.fulfill({ json: answer(query) })
      : route.fulfill({ status, json: { error: "bbox too large" } });
  });
  return served;
}

const MAX_STEPS = 12;

/** Zooms in a step at a time, each step's view settled, until the flights layer asks for one. */
async function zoomUntilAsked(page: Page, served: Served): Promise<void> {
  const zoomIn = page.getByRole("button", { name: "Zoom in" });
  for (let step = 0; step < MAX_STEPS && served.aircraft.length === 0; step += 1) {
    const asked = served.zooms.length;
    // oxlint-disable-next-line no-await-in-loop -- one step after the other
    await zoomIn.click();
    // oxlint-disable-next-line no-await-in-loop -- the step's settled view
    await expect.poll(() => served.zooms.length, { timeout: 10_000 }).toBeGreaterThan(asked);
  }
  await expect.poll(() => served.aircraft.length).toBeGreaterThan(0);
}

const toggle = (page: Page): Locator => page.getByRole("button", { name: "Live flights" });
const legend = (page: Page): Locator => page.locator("[data-flights-legend]");

/** Clicks the middle of the map, where the airliner is drawn. */
async function clickMiddle(page: Page): Promise<void> {
  const box = await page.locator(".maplibregl-canvas").boundingBox();
  await page.mouse.click((box?.x ?? 0) + (box?.width ?? 0) / 2, (box?.y ?? 0) + (box?.height ?? 0) / 2);
}

test.describe("live flights", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("are off at every load, and at world view say to zoom in and ask for nothing", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/map");
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "false");
    await toggle(page).click();
    await expect(legend(page)).toContainText("Flights · zoom in to see them");
    await expect(legend(page)).toContainText("Shown for an area up to about 5° across.");
    expect(served.aircraft).toHaveLength(0);
  });

  test("zoomed in, ask for the view's box only, and count, date and credit what they show", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/map");
    await toggle(page).click();
    await zoomUntilAsked(page, served);
    expect([...(served.aircraft[0]?.keys() ?? [])]).toStrictEqual(["bbox"]);
    await expect(legend(page)).toContainText("Flights · 2 in this view");
    await expect(legend(page)).toContainText(/Positions from just now · \d\d:\d\d UTC/u);
    await expect(legend(page)).toContainText("© adsb.lol contributors · ODbL 1.0");
  });

  test("an aircraft opens its report, and Escape closes it with the map focused", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/map");
    await toggle(page).click();
    await zoomUntilAsked(page, served);
    await expect(legend(page)).toContainText("2 in this view");
    const heading = page.getByRole("heading", { name: "TAP215" });
    // The symbol is placed a frame or two after its data arrives; under load, the first click can come first.
    await expect(async () => {
      await clickMiddle(page);
      await expect(heading).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 10_000 });
    await expect(page.getByText("417 kt (772 km/h)")).toBeVisible();
    await expect(page.getByText(/^Live positions from adsb\.lol/u)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(heading).toHaveCount(0);
  });

  test("turned off, close the aircraft's report", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/map");
    await toggle(page).click();
    await zoomUntilAsked(page, served);
    await expect(legend(page)).toContainText("2 in this view");
    const heading = page.getByRole("heading", { name: "TAP215" });
    await expect(async () => {
      await clickMiddle(page);
      await expect(heading).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 10_000 });
    await toggle(page).click();
    await expect(heading).toHaveCount(0);
    await expect(legend(page)).toHaveCount(0);
  });

  test("turn on with a, and are off again after a reload", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    await expect(toggle(page)).toBeVisible();
    await page.locator("body").press("a");
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "true");
    await expect(legend(page)).toBeVisible();
    await page.reload();
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "false");
    await expect(legend(page)).toHaveCount(0);
  });

  test("a view the server refuses says to zoom in, not that flights failed", async ({ page }) => {
    const served = await serve(page, 400);
    await page.goto("/map");
    await toggle(page).click();
    await zoomUntilAsked(page, served);
    await expect(legend(page)).toContainText("Flights · zoom in to see them");
    await expect(legend(page)).not.toContainText("didn’t load");
  });
});

test.describe("live flights on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("an aircraft opens its report in the sheet; Close gives focus back to the switch", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/map");
    await toggle(page).tap();
    await zoomUntilAsked(page, served);
    await expect(legend(page)).toContainText("2 in this view");
    const heading = page.getByRole("heading", { name: "TAP215" });
    await expect(async () => {
      await clickMiddle(page);
      await expect(heading).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 10_000 });
    await page.getByRole("article", { name: "TAP215" }).getByRole("button", { name: "Close" }).tap();
    await expect(heading).toHaveCount(0);
    await expect(toggle(page)).toBeFocused();
  });

  test("the key sits clear of the controls", async ({ page }) => {
    await serve(page);
    await page.goto("/map");
    await toggle(page).tap();
    await expect(legend(page)).toBeVisible();
    const controls = await toggle(page).boundingBox();
    const key = await legend(page).boundingBox();
    expect((key?.x ?? 0) + (key?.width ?? 0)).toBeLessThanOrEqual(controls?.x ?? 0);
  });
});

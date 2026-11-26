import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const now = (): string => new Date().toISOString();

function quake(id: string, kind: string, severity: string): Record<string, unknown> {
  return {
    id,
    source: "usgs",
    type: "earthquake",
    kind,
    status: "active",
    severity,
    title: `Event ${id}`,
    centroid: { latitude: 0, longitude: 0 },
    countries: [],
    observedAt: now(),
    updatedAt: now(),
    fetchedAt: now(),
    attrs: {},
  };
}

function cluster(count: number, severities: Record<string, number>): Record<string, unknown> {
  return {
    centroid: { latitude: 10, longitude: 10 },
    count,
    maxSeverity: "moderate",
    types: { flood: count },
    severities,
    statuses: {},
    activeSeverities: {},
    bbox: [5, 5, 15, 15],
  };
}

/** The world: a severe event, a warning, and ten clustered; warnings asked apart: one, and four clustered. */
const WORLD = {
  events: [quake("a1", "observed", "severe"), quake("a2", "warning", "minor")],
  clusters: [cluster(10, { moderate: 10 })],
};
const WARNINGS = { events: [quake("a2", "warning", "minor")], clusters: [cluster(4, { minor: 4 })] };

function source(id: string, name: string, enabled = true): Record<string, unknown> {
  return {
    id,
    name,
    url: `https://example.org/${id}`,
    license: "Public domain",
    attribution: `Data: ${name}`,
    pollIntervalSeconds: 300,
    enabled,
  };
}

interface Served {
  /** Each `/v1/events` request's query. */
  events: URLSearchParams[];
  /** Every request to the basemap's tile host: the landing page has no map. */
  tiles: string[];
  /** The map's code and the live stream: neither loads on the landing page. */
  mapOrStream: string[];
}

interface Answers {
  events?: number;
  /** The status of the warnings' own request, when it differs. */
  warnings?: number;
  sources?: number;
  truncated?: boolean;
}

const MAP_OR_STREAM = /useHazardMap|maplibre|\/v1\/stream/u;

async function serve(
  page: Page,
  { events = 200, warnings = events, sources = 200, truncated = false }: Answers = {},
): Promise<Served> {
  const served: Served = { events: [], tiles: [], mapOrStream: [] };
  page.on("request", (request) => {
    if (request.url().startsWith("https://tiles.openfreemap.org/")) {
      served.tiles.push(request.url());
    }
    if (MAP_OR_STREAM.test(request.url())) {
      served.mapOrStream.push(request.url());
    }
  });
  await page.route("**/v1/events?**", (route) => {
    const query = new URL(route.request().url()).searchParams;
    served.events.push(query);
    const asked = query.get("kind") === "warning";
    const status = asked ? warnings : events;
    if (status !== 200) {
      return route.fulfill({ status, json: { error: "unavailable" } });
    }
    const body = asked ? WARNINGS : WORLD;
    return route.fulfill({ json: { ...body, truncated, generatedAt: now() } });
  });
  await page.route("**/v1/sources", (route) =>
    sources === 200
      ? route.fulfill({
          json: { sources: [source("usgs", "USGS Earthquake Hazards Program"), source("old", "Retired feed", false)] },
        })
      : route.fulfill({ status: sources, json: { error: "unavailable" } }),
  );
  return served;
}

const strip = (page: Page): ReturnType<Page["getByRole"]> =>
  page.getByRole("region", { name: "Worldwide · past 24 h" });

test.describe("landing on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("leads with the lockup and the live map, counts the world, and loads no map", async ({ page }) => {
    const served = await serve(page);
    await page.goto("/");
    await expect(page.getByRole("img", { name: "Aegis Terra" })).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(
      "Global hazard intelligence. Official sources, live.",
    );
    // Warnings counted exactly from their own answer, clustered ones included.
    await expect(strip(page)).toContainText(/12\s*Observed events/u);
    await expect(strip(page)).toContainText(/1\s*High severity/u);
    await expect(strip(page)).toContainText(/5\s*Official warnings/u);
    await expect(strip(page)).toContainText("Updated now");
    expect(served.events.map((query) => query.get("kind"))).toContain("warning");
    expect(served.tiles).toHaveLength(0);
    expect(served.mapOrStream).toStrictEqual([]);
  });

  test("a list the server cut short reads as a lower bound", async ({ page }) => {
    await serve(page, { truncated: true });
    await page.goto("/");
    await expect(strip(page)).toContainText(/12\+\s*Observed events/u);
  });

  test("warnings failing alone say why, under the other numbers", async ({ page }) => {
    await serve(page, { warnings: 503 });
    await page.goto("/");
    await expect(strip(page)).toContainText(/12\s*Observed events/u);
    await expect(strip(page)).toContainText(/–\s*Official warnings/u);
    await expect(strip(page)).toContainText("Something went wrong loading events.");
  });

  test("has no serious accessibility problems on a phone", async ({ page }) => {
    await serve(page);
    await page.goto("/");
    await expect(strip(page)).toContainText("Updated now");
    const results = await new AxeBuilder({ page }).analyze();
    const serious = results.violations
      .filter((each) => each.impact === "serious" || each.impact === "critical")
      .map((each) => `${each.id}: ${each.help}`);
    expect(serious).toStrictEqual([]);
  });

  test("names the kinds, lists only the active sources, and closes on the disclaimer", async ({ page }) => {
    await serve(page);
    await page.goto("/");
    const kinds = page.getByRole("list", { name: "Kinds of record" });
    await expect(kinds.getByRole("listitem")).toHaveText([
      "Observed",
      "Official warning",
      "Forecast",
      "Satellite heat detection",
    ]);
    const sources = page.getByRole("region", { name: "Sources" });
    await expect(sources.getByRole("link", { name: /^USGS Earthquake Hazards Program/u })).toBeVisible();
    await expect(sources).toContainText("Data: USGS Earthquake Hazards Program");
    await expect(sources).not.toContainText("Retired feed");
    await expect(page.getByRole("contentinfo")).toContainText(/does not replace your national emergency service/u);
  });

  test("the header's links fit, each a full target", async ({ page }) => {
    await serve(page);
    await page.goto("/");
    const nav = page.getByRole("navigation", { name: "Site" });
    const links = nav.getByRole("link");
    await expect(links).toHaveText(["Live map", "Countries", "Sources"]);
    const each = await links.all();
    const boxes = await Promise.all(each.map((link) => link.boundingBox()));
    expect(boxes.every((box) => (box?.height ?? 0) >= 44 && (box?.x ?? 0) + (box?.width ?? 0) <= 390)).toBe(true);
  });
});

test.describe("landing", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("opens the live map", async ({ page }) => {
    await serve(page);
    await page.goto("/");
    await page.getByRole("link", { name: "Open the live map" }).click();
    await expect(page).toHaveURL(/\/map$/u);
    await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
  });

  test("a failed load shows dashes and says why, never zeros", async ({ page }) => {
    await serve(page, { events: 503, sources: 503 });
    await page.goto("/");
    await expect(strip(page)).toContainText("Something went wrong loading events.");
    await expect(strip(page)).toContainText(/–\s*Observed events/u);
    await expect(strip(page)).not.toContainText(/0\s*Observed/u);
    // No names written into the page when the sources don't load.
    await expect(page.getByRole("region", { name: "Sources" })).toHaveCount(0);
  });

  test("has no serious accessibility problems", async ({ page }) => {
    await serve(page);
    await page.goto("/");
    await expect(strip(page)).toContainText("Updated now");
    const results = await new AxeBuilder({ page }).analyze();
    const serious = results.violations
      .filter((each) => each.impact === "serious" || each.impact === "critical")
      .map((each) => `${each.id}: ${each.help}`);
    expect(serious).toStrictEqual([]);
  });
});

test("an unknown address says so, keeps the site's header and footer, and offers the map", async ({ page }) => {
  await page.goto("/no-such-page");
  await expect(page.getByRole("heading", { name: "There is no page at this address." })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Site" })).toBeVisible();
  await expect(page.getByRole("contentinfo")).toContainText(/does not replace your national emergency service/u);
  await expect(page.getByRole("main").getByRole("link", { name: "Open the live map" })).toBeVisible();
});

test.describe("the site header over the map", () => {
  test("sits above the desktop map, marking where you are", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await serve(page);
    await page.goto("/map");
    const nav = page.getByRole("navigation", { name: "Site" });
    await expect(nav.getByRole("link", { name: "Live map" })).toHaveAttribute("aria-current", "page");
    const header = await page.getByRole("banner").boundingBox();
    const map = await page.getByTestId("hazard-map").boundingBox();
    expect((map?.y ?? 0) >= (header?.y ?? 0) + (header?.height ?? 0)).toBe(true);
  });

  test("keeps the selected event open when a header link changes section", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await serve(page);
    await page.goto("/map?event=a1");
    await page.getByRole("navigation", { name: "Site" }).getByRole("link", { name: "Countries" }).click();
    await expect(page).toHaveURL(/\/countries\?event=a1$/u);
  });

  test("leaves the phone map its whole height", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await serve(page);
    await page.goto("/map");
    await expect(page.getByTestId("hazard-map")).toBeVisible();
    await expect(page.getByRole("navigation", { name: "Site" })).toHaveCount(0);
  });
});

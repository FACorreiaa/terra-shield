import type { Locator, Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const DAY_MS = 86_400_000;

function list(
  events: readonly Record<string, unknown>[],
  clusters: readonly Record<string, unknown>[] = [],
): Record<string, unknown> {
  return { events, clusters, truncated: false, generatedAt: new Date().toISOString() };
}

const QUAKE = {
  id: "6f1c2a7e-0000-4000-8000-000000000001",
  source: "usgs",
  type: "earthquake",
  kind: "observed",
  status: "active",
  severity: "severe",
  sourceSeverity: "M 6.1 mww",
  title: "M 6.1 - 38 km W of Coquimbo, Chile",
  centroid: { latitude: -30.2, longitude: -71.6 },
  countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
  observedAt: new Date(Date.now() - 300_000).toISOString(),
  updatedAt: new Date(Date.now() - 300_000).toISOString(),
  fetchedAt: new Date().toISOString(),
  attrs: {},
};

const CONFLICT_TITLE = "Government of Sudan – RSF";
const CONFLICT = {
  id: "7a2b3c4d-0000-4000-8000-000000000002",
  source: "ucdp",
  type: "conflict",
  kind: "observed",
  status: "active",
  severity: "extreme",
  sourceSeverity: "797 deaths",
  title: CONFLICT_TITLE,
  centroid: { latitude: 13.6, longitude: 25.3 },
  countries: [{ iso2: "SD", relation: "inside" }],
  startedAt: new Date(Date.now() - 40 * DAY_MS).toISOString(),
  observedAt: new Date(Date.now() - 10 * DAY_MS).toISOString(),
  updatedAt: new Date(Date.now() - 10 * DAY_MS).toISOString(),
  fetchedAt: new Date().toISOString(),
  attrs: { locationPrecision: "admin1", violenceType: "state_based" },
};

const CONFLICT_CLUSTER = {
  centroid: { latitude: 49, longitude: 37 },
  count: 14,
  maxSeverity: "severe",
  types: { conflict: 14 },
  severities: { severe: 14 },
  statuses: {},
  activeSeverities: {},
  bbox: [35, 47, 39, 51],
};

interface Served {
  /** Every `/v1/events` request's query, hazards' and conflicts' alike. */
  hazards: URLSearchParams[];
  conflicts: URLSearchParams[];
}

/** Answers hazards and conflicts apart: both are `/v1/events`, the conflicts' with `type=conflict`. */
async function serve(page: Page, hazards: readonly Record<string, unknown>[], conflictStatus = 200): Promise<Served> {
  const served: Served = { hazards: [], conflicts: [] };
  await page.route("**/v1/events?**", (route) => {
    const query = new URL(route.request().url()).searchParams;
    if (query.get("type") === "conflict") {
      served.conflicts.push(query);
      return conflictStatus === 200
        ? route.fulfill({ json: list([CONFLICT], [CONFLICT_CLUSTER]) })
        : route.fulfill({ status: conflictStatus, json: { error: "unavailable" } });
    }
    served.hazards.push(query);
    return route.fulfill({ json: list(hazards) });
  });
  return served;
}

const toggle = (page: Page): Locator => page.getByRole("button", { name: "Armed conflicts" });
const legend = (page: Page): Locator => page.locator("[data-conflict-legend]");
const section = (page: Page): Locator => page.locator("[data-conflict-section]");

test.describe("armed conflicts", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("are off by default, and on ask for conflicts only, with no window", async ({ page }) => {
    const served = await serve(page, [QUAKE]);
    await page.goto("/map");
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "false");
    const summary = page.getByRole("region", { name: "Summary" });
    await expect(summary).toContainText(/1\s*Observed events/u);
    expect(served.conflicts).toHaveLength(0);

    await toggle(page).click();
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "true");
    await expect.poll(() => served.conflicts.length).toBeGreaterThan(0);
    const [query] = served.conflicts;
    expect(query?.get("kind")).toBe("observed");
    expect(query?.get("since")).toBeNull();
    // Hazards are never asked for conflicts, and their counts don't change.
    expect(served.hazards.every((each) => each.get("type") !== "conflict")).toBe(true);
    await expect(legend(page)).toContainText("15 in this view");
    // The extreme conflict counts nowhere in the hazards' strip.
    await expect(summary).toContainText(/1\s*Observed events/u);
    await expect(summary.getByRole("button", { name: /^1\s*High severity/u })).toBeVisible();
  });

  test("show a key that counts what the map grouped, and say what the data are", async ({ page }) => {
    await serve(page, [QUAKE]);
    await page.goto("/map");
    await toggle(page).click();
    await expect(legend(page)).toContainText("Armed conflicts, past 90 days · 15 in this view");
    await expect(legend(page)).toContainText("Conflict data are monthly research estimates (UCDP), not live reports.");
    await expect(
      page.getByRole("button", { name: "Cluster, 14 armed-conflict events, highest severity severe" }),
    ).toBeVisible();
  });

  test("are listed after the hazards, with their dates, and open like any event", async ({ page }) => {
    await serve(page, [QUAKE]);
    await page.goto("/map");
    await toggle(page).click();
    await expect(section(page)).toContainText("Armed conflicts · past 90 days");
    await expect(section(page)).toContainText("14 more grouped on the map; zoom in to list them.");
    const row = section(page).getByRole("button", { name: new RegExp(CONFLICT_TITLE, "u") });
    await expect(row).toContainText("797 deaths");
    // The dashed ring, in words for screen readers.
    await expect(row).toHaveAccessibleName(/approximate location/u);
    await row.click();
    await expect(page.getByRole("heading", { name: CONFLICT_TITLE })).toBeVisible();
  });

  test("with no hazards in view, say so before the conflicts", async ({ page }) => {
    await serve(page, []);
    await page.goto("/map");
    await toggle(page).click();
    await expect(page.getByText("No hazards in this view from covered sources.")).toBeVisible();
    await expect(section(page)).toBeVisible();
  });

  test("turn on and off with c, and stay on across a reload", async ({ page }) => {
    await serve(page, [QUAKE]);
    await page.goto("/map");
    await expect(toggle(page)).toBeVisible();
    await page.locator("body").press("c");
    await expect(legend(page)).toBeVisible();
    await page.reload();
    await expect(toggle(page)).toHaveAttribute("aria-pressed", "true");
    await page.locator("body").press("c");
    await expect(legend(page)).toHaveCount(0);
    await expect(section(page)).toHaveCount(0);
  });

  test("a failed load says so in the key only, announced once", async ({ page }) => {
    await serve(page, [QUAKE], 503);
    await page.goto("/map");
    await toggle(page).click();
    await expect(page.locator("[data-conflict-legend] output")).toHaveText("Armed conflicts didn’t load");
    await expect(page.getByRole("alert")).toHaveCount(0);
  });
});

test.describe("armed conflicts on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("both layer keys sit clear of the controls", async ({ page }) => {
    await serve(page, [QUAKE]);
    await page.goto("/map");
    await toggle(page).tap();
    await page.getByRole("button", { name: "Satellite heat detections" }).tap();
    await expect(legend(page)).toBeVisible();
    const controls = await toggle(page).boundingBox();
    const boxes = await Promise.all(
      [legend(page), page.locator("[data-detection-legend]")].map((key) => key.boundingBox()),
    );
    expect(boxes.map((box) => (box?.x ?? 0) + (box?.width ?? 0) <= (controls?.x ?? 0))).toStrictEqual([true, true]);
  });
});

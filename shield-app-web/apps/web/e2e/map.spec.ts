import { AxeBuilder } from "@axe-core/playwright";

import { expect, test, TILE_HOST } from "./fixtures.ts";

/** One server cluster over Western Europe, so the hazard layer has something to draw. */
const clusterList = {
  events: [],
  clusters: [
    {
      centroid: { latitude: 46.5, longitude: 4.2 },
      count: 14,
      maxSeverity: "extreme",
      types: { flood: 10, wind: 4 },
      severities: { extreme: 2, severe: 5, moderate: 7 },
      statuses: { active: 14 },
      activeSeverities: { extreme: 2, severe: 5, moderate: 7 },
      bbox: [-1.5, 42.1, 9.8, 50.9],
    },
  ],
  truncated: false,
  generatedAt: new Date().toISOString(),
};

const CLUSTER = "Cluster, 14 events, highest severity extreme";

test.beforeEach(async ({ page }) => {
  await page.route("**/v1/events?**", (route) => route.fulfill({ json: clusterList }));
});

test("falls back to the Natural Earth basemap when the tile host is down, hazards still drawn", async ({ page }) => {
  await page.route(TILE_HOST, (route) => route.abort("connectionrefused"));
  await page.goto("/map");
  await expect(page.locator(".maplibregl-ctrl-attrib")).toContainText("Natural Earth");
  await expect(page.getByRole("button", { name: CLUSTER })).toBeVisible();
});

test("credits OpenFreeMap when the tiles load", async ({ page }) => {
  await page.goto("/map");
  await expect(page.locator(".maplibregl-ctrl-attrib")).toContainText("OpenFreeMap");
  await expect(page.locator(".maplibregl-ctrl-attrib")).not.toContainText("Natural Earth");
  await expect(page.getByRole("button", { name: CLUSTER })).toBeVisible();
});

test("keeps the chosen projection across visits", async ({ page }) => {
  await page.goto("/map");
  const layers = page.getByRole("button", { name: "Layers" });
  await layers.click();
  await expect(page.getByRole("radio", { name: "Globe" })).toBeChecked();
  await page.getByText("Flat", { exact: true }).click();
  await expect(page.getByRole("radio", { name: "Flat" })).toBeChecked();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("radio", { name: "Flat" })).toBeHidden();
  await expect(layers).toBeFocused();

  // Closed, the menu leaves Escape alone: focus stays where it is.
  await page.getByRole("button", { name: "Zoom in" }).focus();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Zoom in" })).toBeFocused();

  await page.reload();
  await page.getByRole("button", { name: "Layers" }).click();
  await expect(page.getByRole("radio", { name: "Flat" })).toBeChecked();
});

test("the controls reach the whole world and zoom, with no accessibility violations", async ({ page }) => {
  await page.goto("/map");
  await Promise.all(
    ["Show the whole world", "Zoom in", "Zoom out"].map((name) =>
      expect(page.getByRole("button", { name })).toBeEnabled(),
    ),
  );
  await page.getByRole("button", { name: "Layers" }).click();
  const results = await new AxeBuilder({ page }).include("[data-testid=hazard-map] + div").analyze();
  const serious = results.violations.filter((violation) => ["serious", "critical"].includes(violation.impact ?? ""));
  expect(serious).toStrictEqual([]);
});

import { expect, test } from "./fixtures.ts";

const eventList = {
  events: [
    {
      id: "6f1c2a7e-0000-4000-8000-000000000001",
      source: "usgs",
      type: "earthquake",
      kind: "observed",
      status: "active",
      severity: "severe",
      sourceSeverity: "M 6.1 mww, PAGER yellow",
      title: "M 6.1 - 38 km W of Coquimbo, Chile",
      centroid: { latitude: -30.2, longitude: -71.6 },
      countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
      observedAt: new Date(Date.now() - 12 * 60_000).toISOString(),
      updatedAt: new Date(Date.now() - 5 * 60_000).toISOString(),
      fetchedAt: new Date().toISOString(),
      attrs: {},
    },
  ],
  clusters: [],
  truncated: false,
  generatedAt: new Date().toISOString(),
};

test("shows events from the API with both severities and the disclaimer", async ({ page }) => {
  await page.route("**/v1/events?**", (route) => route.fulfill({ json: eventList }));
  await page.goto("/map");
  await expect(page.getByText("M 6.1 - 38 km W of Coquimbo, Chile")).toBeVisible();
  await expect(page.getByText("Severe", { exact: true })).toBeVisible();
  await expect(page.getByText("M 6.1 mww, PAGER yellow")).toBeVisible();
  await expect(page.getByText("Off the coast of Chile, 38 km")).toBeVisible();
  await expect(page.getByText(/does not replace your national emergency service/u)).toBeVisible();
  await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
});

test("says so when the API is unreachable instead of showing anything", async ({ page }) => {
  await page.route("**/v1/events?**", (route) =>
    route.fulfill({ status: 503, json: { error: true, code: "unavailable", reason: "database down" } }),
  );
  await page.goto("/map");
  await expect(page.getByText("Can’t reach Shield’s API.")).toBeVisible({ timeout: 10_000 });
});

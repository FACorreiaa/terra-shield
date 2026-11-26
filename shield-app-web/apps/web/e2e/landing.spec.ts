import { expect, test } from "@playwright/test";

test("the landing page leads to the live map and carries the disclaimer", async ({ page }) => {
  await page.route("**/v1/**", (route) =>
    route.fulfill({ json: { events: [], clusters: [], truncated: false, generatedAt: new Date().toISOString() } }),
  );
  await page.goto("/");
  await expect(page.getByRole("img", { name: "Aegis Terra" })).toBeVisible();
  await expect(page.getByText(/does not replace your national emergency service/u)).toBeVisible();
  await page.getByRole("link", { name: "Open the live map" }).click();
  await expect(page).toHaveURL(/\/map$/u);
  await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
});

test("an unknown address says so and offers the map", async ({ page }) => {
  await page.goto("/no-such-page");
  await expect(page.getByRole("heading", { name: "There is no page at this address." })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open the live map" })).toBeVisible();
});

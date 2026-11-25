import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures.ts";

const MINUTE_MS = 60_000;

function event(id: string, title: string, minutesAgo: number): Record<string, unknown> {
  return {
    id,
    source: "usgs",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity: "severe",
    sourceSeverity: "M 6.1 mww",
    title,
    centroid: { latitude: -30.2, longitude: -71.6 + minutesAgo },
    countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
    observedAt: new Date(Date.now() - minutesAgo * MINUTE_MS).toISOString(),
    updatedAt: new Date(Date.now() - minutesAgo * MINUTE_MS).toISOString(),
    fetchedAt: new Date().toISOString(),
    attrs: {},
  };
}

const eventList = {
  events: [
    event("6f1c2a7e-0000-4000-8000-000000000001", "M 6.1 - 38 km W of Coquimbo, Chile", 5),
    event("6f1c2a7e-0000-4000-8000-000000000002", "M 6.0 - 90 km W of Valparaiso, Chile", 20),
  ],
  clusters: [],
  truncated: false,
  generatedAt: new Date().toISOString(),
};

/** Serious and critical axe findings inside `selector`. */
async function violations(page: Page, selector: string): Promise<string[]> {
  const results = await new AxeBuilder({ page }).include(selector).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

async function open(page: Page): Promise<void> {
  await page.route("**/v1/events?**", (route) => route.fulfill({ json: eventList }));
  await page.goto("/map");
  await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
}

test.describe("desktop shell", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("selects from the list into the inspector and steps with the keyboard", async ({ page }) => {
    await open(page);
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    await expect(inspector.getByText("M 6.1 - 38 km W of Coquimbo, Chile")).toBeVisible();

    await page.keyboard.press("ArrowDown");
    await expect(inspector.getByText("M 6.0 - 90 km W of Valparaiso, Chile")).toBeVisible();
    expect(await violations(page, "body")).toStrictEqual([]);

    await page.keyboard.press("i");
    await expect(inspector).toHaveCount(0);
    await page.keyboard.press("i");
    await expect(inspector).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(inspector).toHaveCount(0);
  });

  test("lists the shortcuts, and Escape closes that first", async ({ page }) => {
    await open(page);
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    await page.keyboard.press("?");
    const help = page.getByRole("dialog", { name: "Keyboard shortcuts" });
    await expect(help).toBeVisible();
    expect(await violations(page, "dialog")).toStrictEqual([]);

    await page.keyboard.press("Escape");
    await expect(help).toBeHidden();
    await expect(page.getByRole("complementary", { name: "Inspector" })).toBeVisible();
  });

  test("ignores the browser's own shortcuts", async ({ page }) => {
    await open(page);
    await page.getByRole("button", { name: /Coquimbo/u }).click();
    await page.keyboard.press("Control+i");
    await expect(page.getByRole("complementary", { name: "Inspector" })).toBeVisible();
  });
});

test.describe("phone shell", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("walks the sheet's detents from its handle", async ({ page }) => {
    await open(page);
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");

    await handle.click();
    await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
    await handle.press("Enter");
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");
    await handle.press("ArrowDown");
    await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
    await handle.press("ArrowDown");
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
  });

  test("opens a step on a tap anywhere on the sheet's header", async ({ page }) => {
    await open(page);
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    await handle.tap();
    await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
    await page.getByText("Live events", { exact: true }).tap();
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");
  });

  test("throws the sheet to full height with a drag", async ({ page }) => {
    await open(page);
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    const box = await handle.boundingBox();
    expect(box).not.toBeNull();
    const startX = (box?.x ?? 0) + (box?.width ?? 0) / 2;
    const startY = (box?.y ?? 0) + (box?.height ?? 0) / 2;
    await page.mouse.move(startX, startY);
    await page.mouse.down();
    await page.mouse.move(startX, startY - 620, { steps: 8 });
    await page.mouse.up();
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");
  });

  test("opens a selection at half height with the disclaimer and the credits in view", async ({ page }) => {
    await open(page);
    await page.getByRole("button", { name: /^Resize sheet/u }).tap();
    await page.getByRole("button", { name: /Coquimbo/u }).tap();
    await expect(page.getByRole("button", { name: "Close", exact: true })).toBeInViewport();

    const disclaimer = page.getByText(/does not replace your national emergency service/u);
    await disclaimer.scrollIntoViewIfNeeded();
    await expect(disclaimer).toBeInViewport();
    await expect(page.locator(".maplibregl-ctrl-attrib")).toBeInViewport();
    expect(await violations(page, "body")).toStrictEqual([]);
  });
});

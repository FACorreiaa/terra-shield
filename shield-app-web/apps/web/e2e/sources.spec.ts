import { AxeBuilder } from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { event, FIRE, FLOOD, QUAKE, stubCountries } from "./countryStubs.ts";
import { expect, test } from "./fixtures.ts";

const MINUTE_MS = 60_000;

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * MINUTE_MS).toISOString();
}

function source(id: string, name: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    name,
    url: `https://${id}.example.org/`,
    license: "Public domain",
    attribution: name,
    pollIntervalSeconds: 600,
    enabled: true,
    lastSuccessAt: minutesAgo(2),
    stale: false,
    ...extra,
  };
}

// The dot rule (DESIGN.md §10.7, the design session's S3.7 notes): an error older than the last
// success is healthy; a newer one is a failed check; the server's `stale` is stale; disabled hides.
const SOURCES = [
  source("usgs", "USGS Earthquakes", {
    coverage: "Earthquakes of magnitude 2.5 and above, worldwide",
    termsUrl: "https://usgs.example.org/terms",
  }),
  source("eonet", "NASA EONET", { lastErrorAt: minutesAgo(40) }),
  source("gdacs", "GDACS", { lastErrorAt: minutesAgo(1), lastSuccessAt: minutesAgo(30) }),
  source("ucdp", "UCDP", { stale: true, lastSuccessAt: minutesAgo(3000), cadence: "monthly release" }),
  source("old", "Retired feed", { enabled: false }),
];

async function serveSources(page: Page, answers: (number | readonly Record<string, unknown>[])[]): Promise<void> {
  let call = 0;
  await page.route("**/v1/sources", (route) => {
    const answer = answers[Math.min(call, answers.length - 1)] ?? 503;
    call += 1;
    return typeof answer === "number"
      ? route.fulfill({ status: answer, json: { error: "unavailable" } })
      : route.fulfill({ json: { sources: answer } });
  });
}

async function violations(page: Page): Promise<string[]> {
  const results = await new AxeBuilder({ page }).analyze();
  return results.violations
    .filter((each) => each.impact === "serious" || each.impact === "critical")
    .map((each) => `${each.id}: ${each.help}`);
}

test.describe("sources, desktop", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("lists every active source with its health, and the worst on the sidebar's row", async ({ page }) => {
    await serveSources(page, [SOURCES]);
    await page.goto("/map");
    const row = page.getByRole("navigation", { name: "Sources" }).getByRole("link");
    await expect(row).toHaveAccessibleName("Sources Stale");
    await expect(page.getByTestId("hazard-map").locator("canvas")).toBeVisible();
    await page.keyboard.press("5");
    await expect(page).toHaveURL(/\/sources$/u);
    await expect(row).toHaveAttribute("aria-current", "page");

    const list = page.getByRole("region", { name: "4 sources" });
    // Each name follows its health in words, so the dot is never colour alone.
    const headings = list.getByRole("heading", { level: 3 });
    await expect(headings).toHaveCount(4);
    await expect(headings.nth(0)).toHaveAccessibleName("Healthy USGS Earthquakes");
    await expect(headings.nth(1)).toHaveAccessibleName("Healthy NASA EONET");
    await expect(headings.nth(2)).toHaveAccessibleName("Last check failed GDACS");
    await expect(headings.nth(3)).toHaveAccessibleName("Stale UCDP");
    await expect(list.getByText("Retired feed")).toHaveCount(0);
    await expect(list.getByText("Earthquakes of magnitude 2.5 and above, worldwide")).toBeVisible();
    await expect(list.getByText(/^Last check failed 1 min ago · last good check 30 min ago$/u)).toBeVisible();
    await expect(list.getByText(/^Stale · last good check /u)).toBeVisible();
    await expect(list.getByText("Monthly release · checked every 10 min")).toBeVisible();
    await expect(list.getByText("Licence: Public domain")).toHaveCount(4);
    // Terms and the website when the source has terms; else the website alone.
    const usgs = list.getByRole("listitem").filter({ hasText: "USGS" });
    await expect(usgs.getByRole("link")).toHaveText([/^Terms/u, /^Website/u]);
    await expect(usgs.getByRole("link", { name: /^Website.*USGS Earthquakes, opens in a new tab/u })).toBeVisible();
    await expect(list.getByRole("listitem").filter({ hasText: "GDACS" }).getByRole("link")).toHaveText([/^Website/u]);
    await expect(page.getByText("Absence of data is not safety.")).toBeVisible();
    // The map's search steps aside here.
    await expect(page.getByRole("searchbox", { name: "Search countries or events" })).toHaveCount(0);
    expect(await violations(page)).toStrictEqual([]);
  });

  test("fades a stale source's events in the list and says so, and names each source", async ({ page }) => {
    await serveSources(page, [SOURCES]);
    await stubCountries(page, {}, [
      event(QUAKE, "M 4.9 - Antofagasta", { source: "ucdp", severity: "moderate" }),
      event(FIRE, "M 5.2 - Atacama", { source: "ucdp", status: "resolved", endedAt: minutesAgo(180) }),
      event(FLOOD, "M 4.1 - Arica"),
    ]);
    await page.goto("/map");
    const stale = page.getByRole("button", { name: /Antofagasta/u });
    await expect(stale).toContainText(/Stale · last update \d+ (?:h|d) ago/u);
    await expect(stale).toContainText("UCDP");
    await expect(stale.getByText("M 4.9 - Antofagasta")).toHaveClass(/text-muted/u);
    // Resolved says so even when its source is stale.
    const resolved = page.getByRole("button", { name: /Atacama/u });
    await expect(resolved).toContainText("Resolved · ended 3 h ago");
    await expect(resolved).not.toContainText("Stale");
    const fresh = page.getByRole("button", { name: /Arica/u });
    await expect(fresh).toContainText("USGS Earthquakes");
    await expect(fresh.getByText("M 4.1 - Arica")).toHaveClass(/text-ink/u);
  });

  test("shows no dot before the first list", async ({ page }) => {
    await page.route("**/v1/sources", () => {
      // Never answers.
    });
    await page.goto("/map");
    await expect(page.getByRole("navigation", { name: "Sources" }).getByRole("link")).toHaveAccessibleName("Sources");
  });

  test("keeps the last list when a refresh fails", async ({ page }) => {
    await serveSources(page, [SOURCES, 503]);
    await page.goto("/sources");
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
    await page.getByRole("link", { name: "Monitor" }).click();
    // Gone for real, not still on screen while the map's section loads: reopening then refreshes.
    await expect(page.getByRole("region", { name: "4 sources" })).toHaveCount(0);
    await page.keyboard.press("5");
    await expect(page).toHaveURL(/\/sources$/u);
    await expect(page.getByText("Couldn’t refresh. Showing the last list loaded.")).toBeVisible();
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
  });

  test("treats an empty list as a failure, with Try again", async ({ page }) => {
    await serveSources(page, [[], SOURCES]);
    await page.goto("/sources");
    await expect(page.getByRole("alert")).toHaveText("Couldn’t load the sources.");
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
  });

  test("offers Try again when the first load fails", async ({ page }) => {
    await serveSources(page, [503, 503, SOURCES]);
    await page.goto("/sources");
    await expect(page.getByText("Couldn’t load the sources.")).toBeVisible();
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
  });
});

test.describe("sources, phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test("opens from the sheet's header at full height, and Done puts things back", async ({ page }) => {
    await serveSources(page, [SOURCES]);
    await page.goto("/map");
    const handle = page.getByRole("button", { name: /^Resize sheet/u });
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
    await page.getByRole("link", { name: "Sources" }).press("Enter");
    await expect(page).toHaveURL(/\/sources$/u);
    await expect(handle).toHaveAccessibleName("Resize sheet, now full height");
    // Keyboard focus starts on the title; the button that opened it has gone with the field.
    await expect(page.getByRole("heading", { name: "Sources", level: 1 })).toBeFocused();
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
    // The disclaimer once, at the top.
    await expect(page.getByText(/does not replace your national emergency service/u)).toHaveCount(1);
    expect(await violations(page)).toStrictEqual([]);

    await page.getByRole("button", { name: "Done" }).press("Enter");
    await expect(page).toHaveURL(/\/map$/u);
    await expect(handle).toHaveAccessibleName("Resize sheet, now collapsed");
    await expect(page.getByRole("link", { name: "Sources" })).toBeFocused();
  });

  test("closes an open event, and Done comes back to it", async ({ page }) => {
    await serveSources(page, [SOURCES]);
    await stubCountries(page, {}, [event(QUAKE, "M 4.9 - Antofagasta")]);
    await page.goto(`/map?event=${QUAKE}`);
    await expect(page.getByRole("heading", { name: "M 4.9 - Antofagasta" })).toBeVisible();
    await page.getByRole("link", { name: "Sources" }).tap();
    await expect(page).toHaveURL(/\/sources$/u);
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
    await page.getByRole("button", { name: "Done" }).tap();
    await expect(page).toHaveURL(new RegExp(`/map\\?event=${QUAKE}$`, "u"));
    await expect(page.getByRole("heading", { name: "M 4.9 - Antofagasta" })).toBeVisible();

    // Key 5 keeps the event elsewhere; on a phone Sources closes it, or the sheet would hide them.
    await page.keyboard.press("5");
    await expect(page).toHaveURL(/\/sources$/u);
    await expect(page.getByRole("region", { name: "4 sources" })).toBeVisible();
  });

  test("hides the Sources button while searching", async ({ page }) => {
    await serveSources(page, [SOURCES]);
    await page.goto("/map");
    await page.getByRole("searchbox", { name: "Search countries or events" }).tap();
    await expect(page.getByRole("link", { name: "Sources" })).toHaveCount(0);
  });
});

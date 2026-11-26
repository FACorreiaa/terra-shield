import type { BrowserContext, Page } from "@playwright/test";

import { expect, stubApiDefaults, stubTileHost, test } from "./fixtures.ts";
import type { SseServer } from "./sseServer.ts";
import { change, sseServer } from "./sseServer.ts";

const MINUTE_MS = 60_000;
const SHOWN = "6f1c2a7e-0000-4000-8000-0000000000e1";
const ARRIVING = "6f1c2a7e-0000-4000-8000-0000000000e2";
const READY = "id: 41\nevent: ready\ndata: {}\n\n";

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * MINUTE_MS).toISOString();
}

interface Where {
  latitude: number;
  longitude: number;
}

/** An event at `centroid`, as `/v1/events` and the stream send it. */
function event(
  id: string,
  title: string,
  { centroid, ...extra }: { centroid: Where } & Record<string, unknown>,
): Record<string, unknown> {
  return {
    id,
    source: "usgs",
    type: "earthquake",
    kind: "observed",
    status: "active",
    severity: "moderate",
    title,
    centroid,
    countries: [],
    observedAt: minutesAgo(30),
    updatedAt: minutesAgo(10),
    fetchedAt: minutesAgo(1),
    attrs: {},
    ...extra,
  };
}

/** The middle of an events request's view, so an event put there is in view at any zoom. */
function middle(url: string): Where {
  const [west = 0, south = 0, east = 0, north = 0] = (new URL(url).searchParams.get("bbox") ?? "")
    .split(",")
    .map(Number);
  return { latitude: (south + north) / 2, longitude: (west + east) / 2 };
}

interface Served {
  /** Each events request's zoom. */
  zooms: number[];
  /** Adds an event to what later requests answer. */
  add: (title: string) => void;
  where: () => Where;
}

/** Events in the middle of the view: one shown, and any added later. */
async function serveEvents(page: Pick<Page | BrowserContext, "route">): Promise<Served> {
  const zooms: number[] = [];
  const added: string[] = [];
  let where: Where = { latitude: 0, longitude: 0 };
  await page.route("**/v1/events?**", (route) => {
    const url = route.request().url();
    zooms.push(Number(new URL(url).searchParams.get("zoom")));
    where = middle(url);
    const events = [
      event(SHOWN, "M 4.2 - Shown", { centroid: where }),
      ...added.map((title) => event(ARRIVING, title, { centroid: where })),
    ];
    return route.fulfill({ json: { events, clusters: [], truncated: false, generatedAt: new Date().toISOString() } });
  });
  return {
    zooms,
    add: (title) => {
      added.push(title);
    },
    where: () => where,
  };
}

/** Zooms in a step at a time, each step's request answered, until the view's zoom reaches `zoom`. */
async function zoomTo(page: Page, served: Served, zoom: number): Promise<void> {
  const zoomIn = page.getByRole("button", { name: "Zoom in" });
  while ((served.zooms.at(-1) ?? 0) < zoom) {
    const asked = served.zooms.length;
    // oxlint-disable-next-line no-await-in-loop -- one step after the other
    await zoomIn.click();
    // oxlint-disable-next-line no-await-in-loop -- the step's request
    await expect.poll(() => served.zooms.length, { timeout: 10_000 }).toBeGreaterThan(asked);
  }
}

function liveDot(page: Page): ReturnType<Page["locator"]> {
  return page.locator("[data-live]").first();
}

/** `test` with the page's stream served by a test SSE server. */
const liveTest = test.extend<{ server: SseServer }>({
  server: async ({ page }, provide) => {
    const server = await sseServer();
    await server.serve(page);
    await provide(server);
    await server.close();
  },
});

liveTest.describe("live, desktop", () => {
  liveTest.use({ viewport: { width: 1280, height: 800 } });

  liveTest(
    "goes live on ready, reloading so nothing falls between; a change below zoom 6 reloads",
    async ({ page, server }) => {
      const served = await serveEvents(page);
      await page.goto("/map");
      await expect(page.getByRole("button", { name: /M 4\.2 - Shown/u })).toBeVisible();
      await expect(liveDot(page)).toHaveAttribute("data-live", "false");
      await expect.poll(() => server.open()).toBe(1);
      const before = served.zooms.length;

      server.send(READY);
      await expect(liveDot(page)).toHaveAttribute("data-live", "true");
      await expect(page.getByText(/Updated now, live/u).first()).toBeAttached();
      await expect.poll(() => served.zooms.length).toBeGreaterThan(before);

      // Clusters below zoom 6: only the server knows where the new event goes.
      served.add("M 5.0 - Arrived");
      const arrived = event(ARRIVING, "M 5.0 - Arrived", { centroid: served.where() });
      server.send(change(42, "event.created", arrived));
      await expect(page.getByRole("button", { name: /M 5\.0 - Arrived/u })).toBeVisible();
    },
  );

  liveTest("from zoom 6 patches a change in place, with no reload", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect(page.getByRole("button", { name: /M 4\.2 - Shown/u })).toBeVisible();
    await zoomTo(page, served, 6);
    // Settled: the view's last request answered.
    await expect(page.getByRole("button", { name: /M 4\.2 - Shown/u })).toBeVisible();
    await page.waitForTimeout(800);
    const before = served.zooms.length;

    const revised = event(SHOWN, "M 4.4 - Revised", { centroid: served.where() });
    server.send(change(43, "event.updated", revised));
    await expect(page.getByRole("button", { name: /M 4\.4 - Revised/u })).toBeVisible();
    const patchedIn = event(ARRIVING, "M 5.0 - Patched in", { centroid: served.where() });
    server.send(change(44, "event.created", patchedIn));
    await expect(page.getByRole("button", { name: /M 5\.0 - Patched in/u })).toBeVisible();
    server.send(change(45, "event.removed", { id: SHOWN }));
    await expect(page.getByRole("button", { name: /M 4\.4 - Revised/u })).toBeHidden();
    expect(served.zooms.length).toBe(before);
  });

  liveTest("an open event its source removes stays open, as gone", async ({ page, server }) => {
    await serveEvents(page);
    let removed = false;
    await page.route("**/v1/events/*", (route) =>
      removed
        ? route.fulfill({ status: 404, json: { error: "not found" } })
        : route.fulfill({
            json: {
              ...event(SHOWN, "M 4.2 - Shown", { centroid: { latitude: 0, longitude: 0 } }),
              geometry: { type: "Point", coordinates: [0, 0] },
              revisions: [],
            },
          }),
    );
    await page.goto("/map");
    await page.getByRole("button", { name: /M 4\.2 - Shown/u }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: "M 4.2 - Shown" })).toBeVisible();
    removed = true;
    server.send(change(46, "event.removed", { id: SHOWN }));
    await expect(inspector.getByText("This event is no longer published by its source")).toBeVisible();
  });
});

test("one stream serves every tab, and passes on with its cursor when its tab closes", async ({ browser }) => {
  const server = await sseServer();
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  await stubTileHost(context);
  await stubApiDefaults(context);
  await serveEvents(context);
  await server.serve(context);
  const tabs = await Promise.all([context.newPage(), context.newPage(), context.newPage()]);
  for (const tab of tabs) {
    // oxlint-disable-next-line no-await-in-loop -- one tab after the other, so the first leads
    await tab.goto("/map");
  }
  await expect.poll(() => server.open()).toBe(1);
  server.send(READY);
  await Promise.all(tabs.map((tab) => expect(liveDot(tab)).toHaveAttribute("data-live", "true")));

  const [leader] = tabs;
  await leader?.close();
  await expect.poll(() => server.cursors.length).toBe(2);
  expect(server.cursors).toStrictEqual([undefined, "41"]);
  await expect.poll(() => server.open()).toBe(1);
  await context.close();
  await server.close();
});

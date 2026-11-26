import type { Page } from "@playwright/test";

import { expect } from "./fixtures.ts";
import type { Served } from "./liveStubs.ts";
import { ARRIVING, event, liveDot, liveTest, READY, serveEvents, SEVERE_QUAKE, SHOWN } from "./liveStubs.ts";
import type { SseServer } from "./sseServer.ts";
import { change } from "./sseServer.ts";

const TOAST_LINE = /New · Severe earthquake · M6\.1 · Chile/u;

interface Created {
  cursor: number;
  id?: string;
  title: string;
  extra?: Record<string, unknown>;
}

/** An `event.created` message for an event in the middle of the view; severe in Chile unless `extra` says. */
function created(served: Served, { cursor, id = ARRIVING, title, extra = SEVERE_QUAKE }: Created): string {
  return change(cursor, "event.created", event(id, title, { centroid: served.where(), ...extra }));
}

/**
 * Sends `ready` and waits for the reload it brings to land: an event added to the server before
 * then would already be on the map when its `created` comes, and so (rightly) not animate.
 */
async function goLive(page: Page, server: SseServer, served: Served): Promise<void> {
  const before = served.zooms.length;
  server.send(READY);
  await expect(liveDot(page)).toHaveAttribute("data-live", "true");
  await expect.poll(() => served.zooms.length).toBeGreaterThan(before);
}

/** The arriving event's detail, for opening it from the toast. */
function detail(served: Served): Record<string, unknown> {
  const { latitude, longitude } = served.where();
  return {
    ...event(ARRIVING, "M 6.1 - Arrived", { centroid: served.where(), ...SEVERE_QUAKE }),
    geometry: { type: "Point", coordinates: [longitude, latitude] },
    revisions: [],
  };
}

liveTest.describe("arrivals", () => {
  liveTest.use({ viewport: { width: 1280, height: 800 } });

  liveTest("a severe live arrival fades in and is toasted; the toast opens it", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.route("**/v1/events/*", (route) => route.fulfill({ json: detail(served) }));
    await page.goto("/map");
    await expect(page.getByRole("button", { name: /M 4\.2 - Shown/u })).toBeVisible();
    await expect.poll(() => server.open()).toBe(1);
    await goLive(page, server, served);

    served.add("M 6.1 - Arrived");
    // The DOM copy lives under a second: watched for from before the change is sent.
    const copied = page.locator("img.arrive").waitFor({ state: "attached" });
    server.send(created(served, { cursor: 47, title: "M 6.1 - Arrived" }));
    const toast = page.getByRole("button", { name: TOAST_LINE });
    await expect(toast).toBeVisible();
    await expect(page.getByRole("status").filter({ hasText: TOAST_LINE })).toBeAttached();
    // The DOM copy animates in over the map, then hands over to the marker.
    await copied;
    await expect(page.locator("img.arrive")).toHaveCount(0);

    await toast.click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { name: "M 6.1 - Arrived" })).toBeVisible();
    await expect(toast).toBeHidden();
  });

  liveTest("what a connection replays before ready is history: no toast", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect(page.getByRole("button", { name: /M 4\.2 - Shown/u })).toBeVisible();
    await expect.poll(() => server.open()).toBe(1);

    served.add("M 6.1 - Replayed");
    server.send(created(served, { cursor: 40, title: "M 6.1 - Replayed" }));
    server.send(READY);
    await expect(page.getByRole("button", { name: /M 6\.1 - Replayed/u })).toBeVisible();
    await expect(page.getByRole("button", { name: /^New ·/u })).toHaveCount(0);
    await expect(page.locator("img.arrive")).toHaveCount(0);
  });

  liveTest("a moderate arrival gets no toast; a dismissed toast goes", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect.poll(() => server.open()).toBe(1);
    await goLive(page, server, served);

    server.send(created(served, { cursor: 48, title: "M 4.6 - Moderate", extra: {} }));
    server.send(created(served, { cursor: 49, id: SHOWN, title: "M 6.1 - Severe" }));
    const toast = page.getByRole("button", { name: /^New · Severe earthquake/u });
    await expect(toast).toBeVisible();
    await expect(page.getByRole("button", { name: /^New · Moderate/u })).toHaveCount(0);
    await page.getByRole("button", { name: "Dismiss" }).click();
    await expect(toast).toBeHidden();
  });
});

liveTest.describe("arrivals, reduced motion and keyboard", () => {
  liveTest.use({ viewport: { width: 1280, height: 800 } });

  liveTest("under reduced motion the arrival and the toast only fade", async ({ page, server }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect.poll(() => server.open()).toBe(1);
    await goLive(page, server, served);

    served.add("M 6.1 - Arrived");
    // Read in the page the moment the short-lived copy appears.
    const animation = page.waitForFunction(() => {
      const copy = document.querySelector("img.arrive");
      return copy === null ? undefined : getComputedStyle(copy).animationName;
    });
    server.send(created(served, { cursor: 51, title: "M 6.1 - Arrived" }));
    const handle = await animation;
    expect(await handle.jsonValue()).toBe("arrive-fade");
    const toast = page.locator(".toast-in");
    expect(await toast.evaluate((element) => getComputedStyle(element).animationName)).toBe("arrive-fade");
  });

  liveTest("a focused toast stays past 6 s; dismissing it hands focus to Filters", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect.poll(() => server.open()).toBe(1);
    await goLive(page, server, served);

    server.send(created(served, { cursor: 52, title: "M 6.1 - Arrived" }));
    const toast = page.getByRole("button", { name: TOAST_LINE });
    await toast.focus();
    await page.waitForTimeout(6500);
    await expect(toast).toBeVisible();

    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Dismiss" })).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(toast).toBeHidden();
    await expect(page.getByRole("button", { name: /^Filters/u })).toBeFocused();
  });
});

liveTest.describe("arrivals, phone", () => {
  liveTest.use({ viewport: { width: 390, height: 844 } });

  liveTest("the toast sits under the chips, at least 44 px tall, and opens the event", async ({ page, server }) => {
    const served = await serveEvents(page);
    await page.goto("/map");
    await expect.poll(() => server.open()).toBe(1);
    await goLive(page, server, served);

    served.add("M 6.1 - Arrived");
    server.send(created(served, { cursor: 50, title: "M 6.1 - Arrived" }));
    const toast = page.getByRole("button", { name: TOAST_LINE });
    await expect(toast).toBeVisible();
    const box = await toast.boundingBox();

    // Rounded: mid slide-in, the box can measure a hair under its CSS height.
    expect(Math.round(box?.height ?? 0)).toBeGreaterThanOrEqual(44);

    await toast.click();
    await expect(page).toHaveURL(/event=6f1c2a7e-0000-4000-8000-0000000000e2/u);
  });
});

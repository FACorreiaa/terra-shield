import type { BrowserContext, Page } from "@playwright/test";

import { test } from "./fixtures.ts";
import type { SseServer } from "./sseServer.ts";
import { sseServer } from "./sseServer.ts";

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

/** A severe earthquake in Chile, M6.1: what the toast is for. */
const SEVERE_QUAKE = { severity: "severe", attrs: { mag: 6.1 }, countries: [{ iso2: "CL", relation: "within" }] };

export type { Served, Where };
export { ARRIVING, event, liveDot, liveTest, READY, serveEvents, SEVERE_QUAKE, SHOWN };

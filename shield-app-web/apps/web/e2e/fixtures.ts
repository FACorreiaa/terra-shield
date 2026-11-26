import type { BrowserContext, Page } from "@playwright/test";
import { test as base } from "@playwright/test";

/** Every request to the basemap host; tests never reach it (plan: tests never call third-party hosts). */
const TILE_HOST = "https://tiles.openfreemap.org/**";
const TILEJSON_URL = "https://tiles.openfreemap.org/planet";

/** A planet source with no layers: the map loads, draws an empty basemap and never errors. */
const EMPTY_TILEJSON = {
  tilejson: "3.0.0",
  tiles: ["https://tiles.openfreemap.org/planet/{z}/{x}/{y}.pbf"],
  minzoom: 0,
  maxzoom: 14,
  vector_layers: [],
};

type Routable = Pick<Page | BrowserContext, "route">;

/** The tile host for every request: the TileJSON is an empty planet, tiles and glyphs empty bodies. */
async function stubTileHost(target: Routable): Promise<void> {
  await target.route(TILE_HOST, (route) => {
    if (route.request().url() === TILEJSON_URL) {
      return route.fulfill({ json: EMPTY_TILEJSON });
    }
    return route.fulfill({ status: 200, contentType: "application/x-protobuf", body: "" });
  });
}

/**
 * The API calls a test doesn't route itself: no sources yet, an event's detail and a country's
 * summary unavailable, and the live stream refusing for an hour (the page polls, as whenever the
 * stream is down). Nothing reaches the preview's proxy; a test's own route is registered later and wins.
 */
async function stubApiDefaults(target: Routable): Promise<void> {
  await target.route("**/v1/stream", (route) =>
    route.fulfill({ status: 503, headers: { "Retry-After": "3600" }, json: { error: "unavailable" } }),
  );
  await target.route("**/v1/sources", (route) => route.fulfill({ json: { sources: [] } }));
  await target.route("**/v1/events/*", (route) => route.fulfill({ status: 503, json: { error: "unavailable" } }));
  await target.route("**/v1/countries/*/summary", (route) =>
    route.fulfill({ status: 503, json: { error: "unavailable" } }),
  );
}

/** Playwright's `test`, with the tile host and the API's defaults stubbed for every page. */
const test = base.extend<{ stubTiles: undefined; stubApi: undefined }>({
  stubApi: [
    async ({ page }, use) => {
      await stubApiDefaults(page);
      await use(undefined);
    },
    { auto: true },
  ],
  stubTiles: [
    async ({ page }, use) => {
      await stubTileHost(page);
      await use(undefined);
    },
    { auto: true },
  ],
});

export { expect } from "@playwright/test";
export { stubApiDefaults, stubTileHost, test, TILE_HOST };

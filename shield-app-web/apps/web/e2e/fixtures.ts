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

/**
 * Playwright's `test`, with the tile host stubbed for every page: the TileJSON is an empty planet
 * and tiles and glyphs are empty bodies. A test that wants the host down routes it again; the
 * later route wins.
 */
const test = base.extend<{ stubTiles: undefined }>({
  stubTiles: [
    async ({ page }, use) => {
      await page.route(TILE_HOST, (route) => {
        if (route.request().url() === TILEJSON_URL) {
          return route.fulfill({ json: EMPTY_TILEJSON });
        }
        return route.fulfill({ status: 200, contentType: "application/x-protobuf", body: "" });
      });
      await use(undefined);
    },
    { auto: true },
  ],
});

export { expect } from "@playwright/test";
export { test, TILE_HOST };

import { describe, expect, it } from "vite-plus/test";

import { officialUrl } from "./officialSource.ts";

const HOME = { url: "https://earthquake.usgs.gov" };
const PAGE = "https://earthquake.usgs.gov/earthquakes/eventpage/us7000test";

describe(officialUrl, () => {
  it("opens the event's own page", () => {
    expect(officialUrl({ url: PAGE }, HOME, false)).toBe(PAGE);
  });

  it("falls back to the source's home without a page", () => {
    expect(officialUrl({}, HOME, false)).toBe(HOME.url);
    expect(officialUrl({ url: "" }, HOME, false)).toBe(HOME.url);
  });

  it("goes only to the source's home when the event is gone", () => {
    expect(officialUrl({ url: PAGE }, HOME, true)).toBe(HOME.url);
    expect(officialUrl({ url: PAGE }, undefined, true)).toBeUndefined();
  });
});

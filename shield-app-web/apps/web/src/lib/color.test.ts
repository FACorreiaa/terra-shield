import { describe, expect, it } from "vite-plus/test";

import { mix, withAlpha } from "./color.ts";

describe(mix, () => {
  it("returns the ends at 0 and 1 and the midpoint between", () => {
    expect(mix("#000000", "#ffffff", 0)).toBe("#000000");
    expect(mix("#000000", "#ffffff", 1)).toBe("#ffffff");
    expect(mix("#000000", "#ffffff", 0.5)).toBe("#808080");
  });
});

describe(withAlpha, () => {
  it("writes a token as rgb with alpha", () => {
    expect(withAlpha("#52B6FF", 0.18)).toBe("rgb(82 182 255 / 0.18)");
  });
});

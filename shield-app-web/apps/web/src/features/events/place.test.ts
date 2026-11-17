import { describe, expect, it } from "vite-plus/test";

import { place } from "./place.ts";

describe(place, () => {
  it("names a country the event is within", () => {
    expect(place({ countries: [{ iso2: "PT", relation: "within" }] })).toBe("Portugal");
  });

  it("never says 'in' for an offshore event", () => {
    expect(place({ countries: [{ iso2: "CL", relation: "nearby", distanceKm: 37.6 }] })).toBe(
      "Off the coast of Chile, 38 km",
    );
  });

  it("lists every country and nothing for none", () => {
    expect(
      place({
        countries: [
          { iso2: "PT", relation: "within" },
          { iso2: "ES", relation: "within" },
        ],
      }),
    ).toBe("Portugal, Spain");
    expect(place({ countries: [] })).toBeUndefined();
  });
});

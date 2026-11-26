import { describe, expect, it } from "vite-plus/test";

import { countryName, nearbyWords, place } from "./place.ts";
import { sampleEvent } from "./testEvents.ts";

describe(place, () => {
  it("names a country the event is within", () => {
    expect(place({ countries: [{ iso2: "PT", relation: "within" }] })).toBe("Portugal");
  });

  it("never says 'in' for an offshore event", () => {
    expect(place({ countries: [{ iso2: "CL", relation: "nearby", distanceKm: 37.6 }] })).toBe(
      "Off the coast of Chile, 38 km",
    );
    expect(place({ countries: [{ iso2: "CL", relation: "nearby" }] })).toBe("Off the coast of Chile");
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

describe(nearbyWords, () => {
  const quake = sampleEvent(1);
  const flood = sampleEvent(2);

  it("says near for an offshore event", () => {
    expect(nearbyWords(quake, "CL", "Chile")).toBe("Near Chile, 38 km offshore");
    expect(nearbyWords({ countries: [{ iso2: "CL", relation: "nearby" }] }, "CL", "Chile")).toBe(
      "Near Chile, offshore",
    );
  });

  it("says nothing for an event within the country, or not linked to it", () => {
    expect(nearbyWords(flood, "IT", "Italy")).toBeUndefined();
    expect(nearbyWords(flood, "PT", "Portugal")).toBeUndefined();
  });
});

describe(countryName, () => {
  it("falls back to the code for anything that is not one", () => {
    expect(countryName("CL")).toBe("Chile");
    expect(countryName("x1")).toBe("x1");
  });
});

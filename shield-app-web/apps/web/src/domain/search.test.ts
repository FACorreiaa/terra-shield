import { describe, expect, it } from "vite-plus/test";

import { activeEventWords, eventsMatching, resultWords } from "./search.ts";

const events = [
  { id: "a", title: "Flood warning for Côte-d’Or" },
  { id: "b", title: "M 6.1 - 38 km W of Coquimbo, Chile" },
  { id: "c", title: "Wildfire in Chile" },
];

describe(eventsMatching, () => {
  it("matches part of a title, ignoring case and accents, in the order given", () => {
    expect(eventsMatching("chile", events).map((event) => event.id)).toStrictEqual(["b", "c"]);
    expect(eventsMatching("COTE", events).map((event) => event.id)).toStrictEqual(["a"]);
  });

  it("matches nothing for blank text", () => {
    expect(eventsMatching("   ", events)).toStrictEqual([]);
    expect(eventsMatching("tsunami", events)).toStrictEqual([]);
  });
});

describe(activeEventWords, () => {
  it("counts in words", () => {
    expect(activeEventWords(0)).toBe("No active events");
    expect(activeEventWords(1)).toBe("1 active event");
    expect(activeEventWords(12)).toBe("12 active events");
  });
});

describe(resultWords, () => {
  it("counts in words", () => {
    expect(resultWords(0)).toBe("No results");
    expect(resultWords(1)).toBe("1 result");
    expect(resultWords(5)).toBe("5 results");
  });
});

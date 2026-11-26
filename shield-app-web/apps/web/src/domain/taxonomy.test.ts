import { describe, expect, it } from "vite-plus/test";

import { hazardTypeTokens } from "@/design/tokens.gen.ts";

import {
  CATEGORIES,
  CATEGORY_TYPES,
  categoryOf,
  compareSeverity,
  eventCategory,
  eventType,
  HAZARD_TYPES,
  isAtLeast,
  kindLabel,
  kindPluralLabel,
  severityLabel,
  severityRank,
  typeLabel,
} from "./taxonomy.ts";

describe(eventType, () => {
  it("keeps conflict and reads an unknown type as other", () => {
    expect(["tropical_cyclone", "meteor", "conflict"].map((wire) => eventType(wire))).toStrictEqual([
      "tropical_cyclone",
      "other",
      "conflict",
    ]);
  });
});

describe("hazard types", () => {
  it("are every token type, in the apps' order, and never conflict", () => {
    expect(HAZARD_TYPES).toStrictEqual(Object.keys(hazardTypeTokens).filter((type) => type !== "conflict"));
    expect(HAZARD_TYPES).not.toContain("conflict");
  });
});

describe(typeLabel, () => {
  it.each([
    ["earthquake", "Earthquake"],
    ["tropical_cyclone", "Tropical cyclone"],
    ["heat", "Extreme heat"],
    ["conflict", "Armed conflict"],
    ["meteor", "Other hazard"],
  ])("labels %s as %s", (wire, expected) => {
    expect(typeLabel(wire)).toBe(expected);
  });
});

describe(categoryOf, () => {
  it.each([
    ["earthquake", "quakes"],
    ["tsunami", "quakes"],
    ["storm", "weather"],
    ["drought", "weather"],
    ["flood", "floods"],
    ["wildfire", "fires"],
    ["volcano", "volcanoes"],
    ["conflict", "conflicts"],
    ["other", "other"],
    ["meteor", "other"],
  ])("puts %s under %s", (wire, expected) => {
    expect(categoryOf(wire)).toBe(expected);
  });

  it("covers every hazard type in exactly one menu, in menu order", () => {
    expect(CATEGORIES).toStrictEqual(Object.keys(CATEGORY_TYPES));
    const listed = Object.values(CATEGORY_TYPES).flat();
    expect(listed.toSorted()).toStrictEqual([...HAZARD_TYPES, "conflict"].toSorted());
  });
});

describe(eventCategory, () => {
  it("prefers the server's category and reads an unknown one as other", () => {
    expect(eventCategory({ type: "storm", category: "weather" })).toBe("weather");
    expect(eventCategory({ type: "storm", category: "space" })).toBe("other");
    expect(eventCategory({ type: "storm" })).toBe("weather");
  });
});

describe(compareSeverity, () => {
  it("orders minor to extreme", () => {
    expect((["extreme", "minor", "severe", "moderate"] as const).toSorted(compareSeverity)).toStrictEqual([
      "minor",
      "moderate",
      "severe",
      "extreme",
    ]);
    expect(severityRank("extreme")).toBe(3);
  });

  it("compares against a minimum", () => {
    expect(isAtLeast("extreme", "severe")).toBe(true);
    expect(isAtLeast("severe", "severe")).toBe(true);
    expect(isAtLeast("moderate", "severe")).toBe(false);
  });
});

describe("kind and severity words", () => {
  it("use the fixed words of DESIGN.md §2", () => {
    expect(kindLabel("warning")).toBe("Official warning");
    expect(kindLabel("detection")).toBe("Satellite heat detection");
    expect(severityLabel("severe")).toBe("Severe");
  });

  it("plural kind words never call a detection a fire", () => {
    expect(kindPluralLabel("warning")).toBe("warnings");
    expect(kindPluralLabel("detection")).toBe("heat detections");
  });
});

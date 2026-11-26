import type { EventKind, HazardEvent, Severity } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import type { Country, CountrySummary } from "./countries.ts";
import {
  countIn,
  eventsIn,
  highSeverityCount,
  isListCapped,
  mapBox,
  matching,
  matrix,
  normalizedCode,
  smallestArc,
  warningCount,
} from "./countries.ts";

const NOW = "2027-01-15T08:00:00Z";
const UNIT_BOX = [0, 0, 1, 1];

function country(iso2: string, name: string, extra: Partial<Country> = {}): Country {
  return { iso2, iso3: "XXX", name, activeEvents: 0, bbox: UNIT_BOX, ...extra };
}

interface EventOptions {
  type?: string;
  severity?: Severity;
  kind?: EventKind;
  lon?: number;
  lat?: number;
  id?: string;
}

function event({
  type = "flood",
  severity = "moderate",
  kind = "warning",
  lon = 0,
  lat = 0,
  id = "e",
}: EventOptions): HazardEvent {
  return {
    id,
    source: "test",
    type,
    kind,
    status: "active",
    severity,
    title: "e",
    centroid: { latitude: lat, longitude: lon },
    countries: [],
    observedAt: NOW,
    updatedAt: NOW,
    fetchedAt: NOW,
    attrs: {},
  };
}

function summary(activeEvents: number, events: HazardEvent[], counts: CountrySummary["counts"]): CountrySummary {
  return {
    country: country("CH", "Switzerland", { iso3: "CHE", activeEvents }),
    counts,
    coverage: { officialWarnings: true, sources: ["meteoalarm"] },
    events,
    recentChanges: [],
    generatedAt: NOW,
  };
}

describe(normalizedCode, () => {
  it("normalises a code or refuses it", () => {
    expect(normalizedCode(" pt ")).toBe("PT");
    expect(normalizedCode("PRT")).toBeUndefined();
    expect(normalizedCode("P1")).toBeUndefined();
    expect(normalizedCode("")).toBeUndefined();
  });
});

describe(matching, () => {
  const all = [
    country("PT", "Portugal", { iso3: "PRT" }),
    country("PY", "Paraguay", { iso3: "PRY" }),
    country("CI", "Côte d'Ivoire", { iso3: "CIV" }),
    country("PG", "Papua New Guinea", { iso3: "PNG" }),
    country("GN", "Guinea", { iso3: "GIN" }),
  ];
  const codes = (text: string, limit?: number): string[] => matching(text, all, limit).map((match) => match.iso2);

  it("puts codes first, then prefixes, then contains", () => {
    expect(codes("pt")[0]).toBe("PT");
    expect(codes("prt")).toStrictEqual(["PT"]);
    // Accents ignored.
    expect(codes("cote")).toStrictEqual(["CI"]);
    // "Guinea" starts the name before it is found inside "Papua New Guinea".
    expect(codes("guinea")).toStrictEqual(["GN", "PG"]);
  });

  it("lists nothing for blank text, and no more than asked", () => {
    expect(codes("  ")).toStrictEqual([]);
    expect(codes("a", 2)).toHaveLength(2);
  });

  it("lists five at most", () => {
    const many = Array.from({ length: 8 }, (_unused, index) => country(`A${index}`, `Alpha ${index}`));
    expect(matching("alpha", many)).toHaveLength(5);
  });
});

describe(mapBox, () => {
  it("is the bbox of an ordinary country", () => {
    const pt = country("PT", "Portugal", { iso3: "PRT", bbox: [-31.3, 32.6, -6.2, 42.1] });
    expect(mapBox(pt, [])).toStrictEqual({ west: -31.3, south: 32.6, east: -6.2, north: 42.1 });
  });

  it("frames the events of a country across the antimeridian", () => {
    // /v1/countries gives the US [-178.2, 19.0, 179.8, 71.4]: the whole world.
    const us = country("US", "United States", { iso3: "USA", bbox: [-178.2, 19, 179.8, 71.4] });
    const events = [event({ lon: -120, lat: 37 }), event({ lon: -150, lat: 61 }), event({ lon: 179, lat: 52 })];
    // From the Aleutians west of ±180° round to California: crosses the line, 61° wide.
    expect(mapBox(us, events)).toStrictEqual({ west: 179, south: 37, east: -120, north: 61 });
  });

  it("is the band of latitude without events", () => {
    const ru = country("RU", "Russia", { iso3: "RUS", bbox: [-180, 41.2, 180, 81.9] });
    expect(mapBox(ru, [])).toStrictEqual({ west: -180, south: 41.2, east: 180, north: 81.9 });
  });
});

describe(smallestArc, () => {
  it("skips the widest gap", () => {
    expect(smallestArc([10, 20])).toStrictEqual([10, 20]);
    expect(smallestArc([-170, 170])).toStrictEqual([170, -170]);
    expect(smallestArc([5])).toStrictEqual([5, 5]);
  });
});

describe(warningCount, () => {
  it("is a lower bound when the list is capped", () => {
    // Real shape: CH had 157 active, 100 listed.
    const listed = Array.from({ length: 100 }, () => event({ kind: "warning" }));
    const capped = summary(157, listed, [{ type: "wind", severity: "moderate", count: 157 }]);
    expect(isListCapped(capped)).toBe(true);
    expect(warningCount(capped)).toStrictEqual({ value: 100, isLowerBound: true });
    const all = summary(2, [event({ kind: "warning" }), event({ kind: "observed" })], []);
    expect(warningCount(all)).toStrictEqual({ value: 1, isLowerBound: false });
  });
});

describe(highSeverityCount, () => {
  it("comes from the counts", () => {
    const counts: CountrySummary["counts"] = [
      { type: "wind", severity: "extreme", count: 2 },
      { type: "flood", severity: "severe", count: 3 },
      { type: "flood", severity: "moderate", count: 4 },
    ];
    expect(highSeverityCount(summary(9, [], counts))).toBe(5);
  });
});

describe(matrix, () => {
  it("folds unknown types into other and sorts by severity", () => {
    // Two unknown wire types share the `other` row.
    const tenEvents = summary(
      10,
      [],
      [
        { type: "wind", severity: "moderate", count: 6 },
        { type: "landslide", severity: "minor", count: 1 },
        { type: "sinkhole", severity: "minor", count: 2 },
        { type: "flood", severity: "severe", count: 1 },
      ],
    );
    const rows = matrix(tenEvents);
    expect(rows.map((row) => row.type)).toStrictEqual(["flood", "wind", "other"]);
    expect(rows.at(-1)?.counts.minor).toBe(3);
    expect(countIn(tenEvents, { type: "other", severity: "minor" })).toBe(3);
  });

  it("breaks ties by the most events, then the name", () => {
    const rows = matrix(
      summary(
        6,
        [],
        [
          { type: "wind", severity: "moderate", count: 1 },
          { type: "storm", severity: "moderate", count: 1 },
          { type: "heat", severity: "moderate", count: 4 },
        ],
      ),
    );
    expect(rows.map((row) => [row.type, row.total, row.maxSeverity])).toStrictEqual([
      ["heat", 4, "moderate"],
      ["storm", 1, "moderate"],
      ["wind", 1, "moderate"],
    ]);
  });
});

describe(eventsIn, () => {
  it("filters the list to a cell", () => {
    const flood = event({ id: "a", type: "flood", severity: "severe" });
    const wind = event({ id: "b", type: "wind", severity: "severe" });
    const minor = event({ id: "c", type: "flood", severity: "minor" });
    expect(
      eventsIn([flood, wind, minor], { type: "flood", severity: "severe" }).map((listed) => listed.id),
    ).toStrictEqual(["a"]);
    expect(eventsIn([flood, wind, minor])).toHaveLength(3);
  });
});

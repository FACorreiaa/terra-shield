import type { Aircraft, AircraftList } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import {
  aircraftFigures,
  aircraftNotes,
  aircraftTitle,
  flightsAgeLine,
  flightsAttribution,
  flightsHeadline,
  flightsLines,
} from "./aircraft.ts";

// The cases of shield-core `AircraftWordsTests`.
const NOW = new Date("2026-11-26T11:06:50Z");

function plane(hex: string, changes: Partial<Aircraft> = {}): Aircraft {
  return {
    hex,
    callsign: "TAP215",
    registration: "CS-TXK",
    typeCode: "A21N",
    lat: 38.7,
    lon: -9.1,
    altitudeFt: 38_000,
    onGround: false,
    groundSpeedKt: 417.3,
    trackDeg: 275,
    verticalRateFpm: -128,
    squawk: "2205",
    military: false,
    positionAt: NOW.toISOString(),
    ...changes,
  };
}

function layer(aircraft: Aircraft[], changes: Partial<AircraftList> = {}): AircraftList {
  return {
    aircraft,
    dataAt: NOW.toISOString(),
    complete: true,
    attribution: "© adsb.lol contributors",
    license: "ODbL 1.0",
    licenseUrl: "https://opendatacommons.org/licenses/odbl/1-0/",
    ...changes,
  };
}

const SECOND_MS = 1000;

function ago(seconds: number): string {
  return new Date(NOW.getTime() - seconds * SECOND_MS).toISOString();
}

describe(aircraftTitle, () => {
  it("is the callsign, else the registration, else the address", () => {
    expect(aircraftTitle(plane("49530a"))).toBe("TAP215");
    expect(aircraftTitle(plane("49530a", { callsign: "  " }))).toBe("CS-TXK");
    const { registration: _unused, ...noRegistration } = plane("49530a", { callsign: "  " });
    expect(aircraftTitle(noRegistration)).toBe("49530A");
  });
});

describe(aircraftFigures, () => {
  it("gives aviation units with metric after", () => {
    expect(aircraftFigures(plane("49530a"))).toStrictEqual([
      { label: "Altitude", value: "38,000 ft (11,582 m)" },
      { label: "Ground speed", value: "417 kt (773 km/h)" },
      { label: "Track", value: "275°" },
      { label: "Vertical rate", value: "−128 ft/min" },
      { label: "Type", value: "A21N" },
      { label: "Registration", value: "CS-TXK" },
      { label: "Squawk", value: "2205" },
      { label: "ICAO address", value: "49530A" },
    ]);
  });

  it("says on the ground, and leaves out what the feed didn't report", () => {
    const grounded: Aircraft = {
      hex: "~2a0001",
      registration: "CS-TXK",
      lat: 38.7,
      lon: -9.1,
      onGround: true,
      military: false,
      positionAt: NOW.toISOString(),
    };
    const figures = aircraftFigures(grounded);
    expect(figures.map((figure) => figure.label)).toStrictEqual(["Altitude", "Registration", "Address (non-ICAO)"]);
    expect(figures[0]?.value).toBe("On the ground");
    // A "~" address is not an ICAO one.
    expect(figures.at(-1)).toStrictEqual({ label: "Address (non-ICAO)", value: "~2A0001" });
  });
});

describe(aircraftNotes, () => {
  it("puts flags first, then the position's age, and the track wraps at 360", () => {
    const flagged = plane("49530a", { trackDeg: 359.6, military: true, emergency: "minfuel", positionAt: ago(40) });
    expect(aircraftNotes(flagged, NOW)).toStrictEqual([
      "Emergency: minfuel",
      "Military (as the feed flags it)",
      "Position received 40 s ago · 11:06 UTC",
    ]);
    expect(aircraftFigures(flagged).find((figure) => figure.label === "Track")?.value).toBe("0°");
    expect(aircraftNotes(plane("x"), NOW)).toStrictEqual(["Position received just now · 11:06 UTC"]);
  });
});

describe(flightsAgeLine, () => {
  it("gives the age of the oldest data, never 'from now'", () => {
    const older = { dataAt: ago(180) };
    const recent = { dataAt: ago(5) };
    expect(flightsAgeLine(older, NOW)).toBe("Positions from 3 min ago · 11:03 UTC");
    expect(flightsAgeLine({}, NOW)).toBe("No recent positions here");
    expect(flightsAgeLine(recent, NOW)).toBe("Positions from just now · 11:06 UTC");
  });
});

describe(flightsHeadline, () => {
  const none = { list: undefined, failed: false, needsZoom: false };

  it("says why there are none to count", () => {
    expect(flightsHeadline({ ...none, needsZoom: true })).toBe("Flights · zoom in to see them");
    expect(flightsHeadline(none)).toBe("Flights · loading…");
    expect(flightsHeadline({ ...none, failed: true })).toBe("Flights didn’t load");
  });

  it("counts the aircraft in view", () => {
    const empty = layer([]);
    const one = layer([plane("a")]);
    const two = layer([plane("a"), plane("b")]);
    expect(flightsHeadline({ ...none, list: empty })).toBe("Flights · none in this view");
    expect(flightsHeadline({ ...none, list: one })).toBe("Flights · 1 in this view");
    expect(flightsHeadline({ ...none, list: two })).toBe("Flights · 2 in this view");
  });
});

describe(flightsLines, () => {
  it("says how old, what is missing, and when a refresh failed", () => {
    const list = layer([plane("a")], { complete: false, dataAt: ago(180) });
    expect(flightsLines({ list, failed: true, needsZoom: false }, NOW)).toStrictEqual([
      "Positions from 3 min ago · 11:03 UTC",
      "Incomplete: part of this view has no recent data.",
      "The last refresh failed; these positions are older.",
    ]);
    expect(flightsLines({ list: undefined, failed: false, needsZoom: true }, NOW)).toStrictEqual([
      "Shown for an area up to about 5° across.",
    ]);
    expect(flightsAttribution(undefined)).toBe("© adsb.lol contributors · ODbL 1.0");
  });
});

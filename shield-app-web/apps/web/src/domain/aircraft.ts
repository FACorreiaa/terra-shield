import type { Aircraft, AircraftList } from "@shield/api-client";

import { formatNumber } from "./attrs.ts";
import { relative, utcClock } from "./freshness.ts";

/**
 * The words of the live flights layer (ADR 28), as `Aircraft.swift` and `Flights.swift` say them:
 * the aircraft's title, its figures and notes, the legend, and the marker's spoken label. A
 * position report, never a flight page: no route, no history.
 */

/** One line of the aircraft's detail. */
interface AircraftFigure {
  label: string;
  value: string;
}

const METRES_PER_FOOT = 0.3048;
const KMH_PER_KNOT = 1.852;
const FULL_TURN = 360;

/** The callsign, else the registration, else the address. */
function aircraftTitle(aircraft: Pick<Aircraft, "callsign" | "registration" | "hex">): string {
  for (const name of [aircraft.callsign, aircraft.registration]) {
    const trimmed = name?.trim() ?? "";
    if (trimmed !== "") {
      return trimmed;
    }
  }
  return aircraft.hex.toUpperCase();
}

/** The track in whole degrees: 359.6 rounds to 360, which is north, so 0. */
function wholeTrack(trackDeg: number): number {
  return Math.round(trackDeg) % FULL_TURN;
}

function altitudeFigure({ onGround, altitudeFt }: Aircraft): AircraftFigure[] {
  if (onGround) {
    return [{ label: "Altitude", value: "On the ground" }];
  }
  if (altitudeFt === undefined) {
    return [];
  }
  const metres = formatNumber(altitudeFt * METRES_PER_FOOT);
  return [{ label: "Altitude", value: `${formatNumber(altitudeFt)} ft (${metres} m)` }];
}

function motionFigures({ groundSpeedKt, trackDeg, verticalRateFpm }: Aircraft): AircraftFigure[] {
  const out: AircraftFigure[] = [];
  if (groundSpeedKt !== undefined) {
    const kmh = formatNumber(groundSpeedKt * KMH_PER_KNOT);
    out.push({ label: "Ground speed", value: `${formatNumber(groundSpeedKt)} kt (${kmh} km/h)` });
  }
  if (trackDeg !== undefined) {
    out.push({ label: "Track", value: `${wholeTrack(trackDeg)}°` });
  }
  if (verticalRateFpm !== undefined) {
    // A true minus sign: it is read as a figure, not a hyphen.
    const sign = verticalRateFpm < 0 ? "−" : "";
    out.push({ label: "Vertical rate", value: `${sign}${formatNumber(Math.abs(verticalRateFpm))} ft/min` });
  }
  return out;
}

function identityFigures({ typeCode, registration, squawk, hex }: Aircraft): AircraftFigure[] {
  const named: [string, string | undefined][] = [
    ["Type", typeCode],
    ["Registration", registration],
    ["Squawk", squawk],
  ];
  const out = named
    .filter((entry): entry is [string, string] => entry[1] !== undefined && entry[1] !== "")
    .map(([label, value]) => ({ label, value }));
  // A leading "~" marks an address that isn't an ICAO one.
  const icao = !hex.startsWith("~");
  out.push({ label: icao ? "ICAO address" : "Address (non-ICAO)", value: hex.toUpperCase() });
  return out;
}

/** Aviation units first, metric after; lines the feed did not report are left out. */
function aircraftFigures(aircraft: Aircraft): AircraftFigure[] {
  return [...altitudeFigure(aircraft), ...motionFigures(aircraft), ...identityFigures(aircraft)];
}

/** `freshness.label`, except under ten seconds: "from now" reads as the future, so "just now". */
function positionAge(date: Date, now: Date): string {
  const ago = relative(date, now);
  return `${ago === "now" ? "just now" : ago} · ${utcClock(date)}`;
}

/** Lines above the figures: an emergency first, the military flag, how old this position is. */
function aircraftNotes(aircraft: Aircraft, now: Date): string[] {
  const out: string[] = [];
  if (aircraft.emergency !== undefined && aircraft.emergency !== "") {
    out.push(`Emergency: ${aircraft.emergency}`);
  }
  if (aircraft.military) {
    out.push("Military (as the feed flags it)");
  }
  out.push(`Position received ${positionAge(new Date(aircraft.positionAt), now)}`);
  return out;
}

/** "Positions from 3 min ago · 08:23 UTC": the oldest data in view, never "live". */
function flightsAgeLine(list: Pick<AircraftList, "dataAt">, now: Date): string {
  return list.dataAt === undefined
    ? "No recent positions here"
    : `Positions from ${positionAge(new Date(list.dataAt), now)}`;
}

/** What the flights legend knows. */
interface FlightsState {
  list: AircraftList | undefined;
  failed: boolean;
  /** The view is larger than the server serves (four 5° cells). */
  needsZoom: boolean;
}

/** "Flights · 12 in this view", or why there are none to count. */
function flightsHeadline({ list, failed, needsZoom }: FlightsState): string {
  if (needsZoom) {
    return "Flights · zoom in to see them";
  }
  if (list === undefined) {
    return failed ? "Flights didn’t load" : "Flights · loading…";
  }
  const count = list.aircraft.length;
  if (count === 0) {
    return "Flights · none in this view";
  }
  // A plain count, as the apps interpolate it.
  return `Flights · ${String(count)} in this view`;
}

/** The legend's lines under its headline: how old, whether complete, whether the last refresh failed. */
function flightsLines({ list, failed, needsZoom }: FlightsState, now: Date): string[] {
  if (needsZoom) {
    return ["Shown for an area up to about 5° across."];
  }
  if (list === undefined) {
    return [];
  }
  const out = [flightsAgeLine(list, now)];
  if (!list.complete) {
    out.push("Incomplete: part of this view has no recent data.");
  }
  if (failed) {
    out.push("The last refresh failed; these positions are older.");
  }
  return out;
}

/** The ODbL line, shown whenever the layer is on: the answer's, else adsb.lol's own. */
function flightsAttribution(list: AircraftList | undefined): string {
  return `${list?.attribution ?? "© adsb.lol contributors"} · ${list?.license ?? "ODbL 1.0"}`;
}

export type { AircraftFigure, FlightsState };
export {
  aircraftFigures,
  aircraftNotes,
  aircraftTitle,
  flightsAgeLine,
  flightsAttribution,
  flightsHeadline,
  flightsLines,
  wholeTrack,
};

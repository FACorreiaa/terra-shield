import { describe, expect, it } from "vite-plus/test";

import { detailTimes, keyFigures, locationNote, severityNote } from "./eventFacts.ts";
import { AT, makeEvent } from "./testEvents.ts";

/** shield-core TsunamiDetailTests (S1.11, #25). */

const forecast = (band = "1–3 m", provisional = false): ReturnType<typeof makeEvent> =>
  makeEvent({
    source: "tsunami",
    type: "tsunami",
    kind: "forecast",
    severity: "severe",
    severityProvisional: provisional,
    attrs: {
      center: "PTWC",
      product: "WEPA40",
      bulletin: 3,
      incident: "x",
      epicenter: [160, 52.5],
      geometry: "epicenter",
      magnitude: 8.8,
      magnitudeType: "Mww",
      guidance: true,
      forecast: [
        { band, severity: "severe", places: ["Japan", "Russia"], countries: ["JP", "RU"] },
        { band: "0.3–1 m", severity: "moderate", places: ["Hawaii"], countries: ["US"] },
      ],
      unmatchedPlaces: ["Wake Island"],
    },
  });

const warning = makeEvent({
  source: "tsunami",
  type: "tsunami",
  kind: "warning",
  severity: "extreme",
  attrs: {
    center: "NTWC",
    product: "WEAK51",
    bulletin: 2,
    incident: "y",
    epicenter: [160, 52.5],
    geometry: "zones",
    magnitude: 8.8,
    zones: ["AKZ185", "AKZ187"],
    areas: ["Chignik Bay, Alaska to Samalga Pass, Alaska", "Samalga Pass, Alaska to Attu, Alaska"],
    zonesUnresolved: ["PZZ530", "BCZ921"],
  },
});

describe("tsunami severity note", () => {
  it("calls a forecast guidance, not a warning", () => {
    expect(severityNote(forecast())).toBe("Guidance for national authorities, not a warning.");
    expect(severityNote(warning)).toBeUndefined();
  });

  it("says the level is provisional before wave heights exist", () => {
    expect(severityNote(forecast("hazardous waves possible", true))).toBe(
      "Guidance for national authorities, not a warning. Rated severe until wave heights are forecast.",
    );
  });
});

describe("tsunami key figures", () => {
  it("lead a forecast with its highest band, then each band's places", () => {
    const figures = keyFigures(forecast());
    expect(figures.map((figure) => figure.label)).toStrictEqual([
      "Highest waves",
      "1–3 m",
      "0.3–1 m",
      "Also listed",
      "Earthquake",
      "Issued by",
      "Bulletin",
    ]);
    expect(figures[0]).toStrictEqual({ label: "Highest waves", value: "1–3 m", isLead: true });
    expect(figures.map((figure) => figure.value).slice(1)).toStrictEqual([
      "Japan, Russia",
      "Hawaii",
      "Wake Island",
      "M8.8 Mww",
      "Pacific Tsunami Warning Center",
      "No. 3",
    ]);
  });

  it("list a US warning's coasts and what the map cannot draw", () => {
    const figures = keyFigures(warning);
    expect(figures.map((figure) => figure.label)).toStrictEqual([
      "Coasts",
      "Not on the map",
      "Earthquake",
      "Issued by",
      "Bulletin",
    ]);
    // Segment descriptions hold commas of their own, so they are set apart by semicolons; marine
    // zones (PZZ…) are never drawn, so they aren't counted as missing.
    expect(figures.map((figure) => figure.value).slice(0, 4)).toStrictEqual([
      "Chignik Bay, Alaska to Samalga Pass, Alaska; Samalga Pass, Alaska to Attu, Alaska",
      "1 zone without a shape (BCZ921)",
      "M8.8",
      "National Tsunami Warning Center",
    ]);
  });

  it("say nothing of marine zones alone", () => {
    const marine = { ...warning, attrs: { ...warning.attrs, zonesUnresolved: ["PZZ530", "AMZ650", "GMZ032"] } };
    expect(keyFigures(marine).map((figure) => figure.label)).not.toContain("Not on the map");
  });
});

describe("tsunami times and place", () => {
  it("show a forecast's expiry as Shield's safety net, not the centre's", () => {
    const expiring = { expiresAt: "2026-11-16T18:00:00Z" };
    const net = forecast();
    expect(detailTimes({ ...net, attrs: { ...net.attrs, ...expiring } }, AT).at(-1)?.label).toBe("Shown until");
    expect(detailTimes({ ...warning, attrs: { ...warning.attrs, ...expiring } }, AT).at(-1)?.label).toBe("Expires");
  });

  it("say where the waves are not for an epicentre marker", () => {
    expect(locationNote(forecast())).toBe(
      "Shown at the earthquake’s epicentre. The waves reach the places listed below.",
    );
    expect(locationNote(warning)).toBeUndefined();
  });
});

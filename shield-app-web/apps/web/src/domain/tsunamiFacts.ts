import type { Attrs, KeyFigure } from "./attrs.ts";
import { attrArray, attrNumber, attrString, attrStrings, formatMagnitude, isRecord } from "./attrs.ts";

/** NOAA tsunami warning centres (docs/attrs.md "Tsunami", #25). */

/** One band of a PTWC threat forecast, most severe first. */
interface TsunamiBand {
  readonly band: string;
  readonly places: readonly string[];
}

/** NWS marine forecast areas (UGC prefixes): coastal waters, the Great Lakes, St Lawrence. */
const MARINE_UGC_PREFIXES: ReadonlySet<string> = new Set([
  "AM",
  "AN",
  "GM",
  "LC",
  "LE",
  "LH",
  "LM",
  "LO",
  "LS",
  "PH",
  "PK",
  "PM",
  "PS",
  "PZ",
  "SL",
]);
const UGC_PREFIX_LENGTH = 2;

const CENTRES: ReadonlyMap<string, string> = new Map([
  ["PTWC", "Pacific Tsunami Warning Center"],
  ["NTWC", "National Tsunami Warning Center"],
]);

/** The forecast's bands; items without a string `band` are skipped. */
function tsunamiBands(attrs: Attrs): TsunamiBand[] {
  return (attrArray(attrs, "forecast") ?? []).flatMap((item) => {
    if (!isRecord(item)) {
      return [];
    }
    const band = attrString(item, "band");
    return band === undefined ? [] : [{ band, places: attrStrings(item, "places") }];
  });
}

/** "1 zone without a shape (BCZ921)": land zones only, since marine zones are never drawn. */
function unresolvedFigure(attrs: Attrs): KeyFigure[] {
  const unresolved = attrStrings(attrs, "zonesUnresolved").filter(
    (zone) => !MARINE_UGC_PREFIXES.has(zone.slice(0, UGC_PREFIX_LENGTH)),
  );
  if (unresolved.length === 0) {
    return [];
  }
  const noun = unresolved.length === 1 ? "zone" : "zones";
  const value = `${unresolved.length} ${noun} without a shape (${unresolved.join(", ")})`;
  return [{ label: "Not on the map", value, isLead: false }];
}

/** The earthquake, the issuing centre and the bulletin number. */
function sourceFigures(attrs: Attrs): KeyFigure[] {
  const out: KeyFigure[] = [];
  const magnitude = attrNumber(attrs, "magnitude");
  if (magnitude !== undefined) {
    const magnitudeType = attrString(attrs, "magnitudeType");
    const suffix = magnitudeType === undefined ? "" : ` ${magnitudeType}`;
    out.push({ label: "Earthquake", value: `M${formatMagnitude(magnitude)}${suffix}`, isLead: false });
  }
  const centre = attrString(attrs, "center");
  if (centre !== undefined) {
    out.push({ label: "Issued by", value: CENTRES.get(centre) ?? centre, isLead: false });
  }
  const bulletin = attrNumber(attrs, "bulletin");
  if (bulletin !== undefined) {
    out.push({ label: "Bulletin", value: `No. ${Math.trunc(bulletin)}`, isLead: false });
  }
  return out;
}

/** A tsunami's figures: highest band, each band's places, coasts, then the source's own facts. */
function tsunamiFigures(attrs: Attrs): KeyFigure[] {
  const bands = tsunamiBands(attrs);
  const [top] = bands;
  const lead: KeyFigure[] = top === undefined ? [] : [{ label: "Highest waves", value: top.band, isLead: true }];
  const places = bands
    .filter((band) => band.places.length > 0)
    .map((band) => ({ label: band.band, value: band.places.join(", "), isLead: false }));
  const unmatched = attrStrings(attrs, "unmatchedPlaces");
  // US segments ("Chignik Bay, Alaska to Samalga Pass, Alaska") carry commas of their own.
  const areas = attrStrings(attrs, "areas");
  return [
    ...lead,
    ...places,
    ...(unmatched.length > 0 ? [{ label: "Also listed", value: unmatched.join(", "), isLead: false }] : []),
    ...(areas.length > 0 ? [{ label: "Coasts", value: areas.join("; "), isLead: false }] : []),
    ...unresolvedFigure(attrs),
    ...sourceFigures(attrs),
  ];
}

/** The first band's name ("hazardous waves possible"), if the forecast has one. */
function topBand(attrs: Attrs): string | undefined {
  return tsunamiBands(attrs)[0]?.band;
}

export { tsunamiFigures, topBand };

/**
 * Typed reads of `Event.attrs` (docs/attrs.md), as strict as shield-core's JSONValue: a value of
 * another JSON type reads as absent, never coerced.
 */

type Attrs = Readonly<Record<string, unknown>>;

/** One type-specific fact: "Magnitude 6.1 mww", "Depth 10 km". */
interface KeyFigure {
  readonly label: string;
  readonly value: string;
  /** The figure that leads (shown large): an earthquake's magnitude, a cyclone's wind. */
  readonly isLead: boolean;
}

/** One of an event's times, labelled: "Started", "Updated", "Expires". */
interface DetailTime {
  readonly label: string;
  readonly date: Date;
  /** Shown as a day ("10 Sep"), never as "N ago": a conflict's dates are months-old estimates. */
  readonly dateOnly: boolean;
}

/** A candidate figure: dropped when its value is absent or empty; lead only if it ends up first. */
type FigureRow = readonly [label: string, value: string | undefined, lead?: boolean];

const numberFormat = new Intl.NumberFormat("en-US", { maximumFractionDigits: 0, roundingMode: "halfEven" });
/** Magnitudes read with one decimal: "6.1". */
const MAGNITUDE_DECIMALS = 1;
/** As `%.1f`: ties round to even (4.25 → "4.2"), which `toFixed` would round up. */
const magnitudeFormat = new Intl.NumberFormat("en-US", {
  minimumFractionDigits: MAGNITUDE_DECIMALS,
  maximumFractionDigits: MAGNITUDE_DECIMALS,
  roundingMode: "halfEven",
  useGrouping: false,
});

function attrString(attrs: Attrs, key: string): string | undefined {
  const value = attrs[key];
  return typeof value === "string" ? value : undefined;
}

function attrNumber(attrs: Attrs, key: string): number | undefined {
  const value = attrs[key];
  return typeof value === "number" ? value : undefined;
}

/** A number read as Swift's `Int(double)`: truncated toward zero. */
function attrInt(attrs: Attrs, key: string): number | undefined {
  const value = attrNumber(attrs, key);
  return value === undefined ? undefined : Math.trunc(value);
}

function attrBool(attrs: Attrs, key: string): boolean | undefined {
  const value = attrs[key];
  return typeof value === "boolean" ? value : undefined;
}

/** The array at `key`, or undefined when it is no array. */
function attrArray(attrs: Attrs, key: string): readonly unknown[] | undefined {
  const value: unknown = attrs[key];
  if (!Array.isArray(value)) {
    return undefined;
  }
  const items: readonly unknown[] = value;
  return items;
}

/** The string elements of an array attr; [] when it is no array. */
function attrStrings(attrs: Attrs, key: string): string[] {
  return (attrArray(attrs, key) ?? []).filter((item): item is string => typeof item === "string");
}

/** A JSON object, as opposed to an array or null. */
function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Whole numbers with grouping, in the app's one language: 48484 → "48,484". */
function formatNumber(value: number): string {
  return numberFormat.format(value);
}

/** One decimal, as `String(format: "%.1f")`: 6.1 → "6.1". */
function formatMagnitude(value: number): string {
  return magnitudeFormat.format(value);
}

/** As Swift's `capitalized`: "green" → "Green", "under review" → "Under Review". */
function capitalizeWords(text: string): string {
  return text
    .toLowerCase()
    .split(" ")
    .map((word) => `${word.slice(0, 1).toUpperCase()}${word.slice(1)}`)
    .join(" ");
}

/** The rows that have a value, the lead flag kept only on the first. */
function figures(rows: readonly FigureRow[]): KeyFigure[] {
  return rows
    .filter(([, value]) => value !== undefined && value !== "")
    .map(([label, value = "", lead = false], index) => ({ label, value, isLead: lead && index === 0 }));
}

/** An RFC 3339 attr as a date; undefined when absent or unreadable. */
function attrDate(attrs: Attrs, key: string): Date | undefined {
  const raw = attrString(attrs, key);
  if (raw === undefined) {
    return undefined;
  }
  const date = new Date(raw);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

export type { Attrs, DetailTime, FigureRow, KeyFigure };
export {
  attrArray,
  attrBool,
  attrDate,
  attrInt,
  attrNumber,
  attrString,
  attrStrings,
  capitalizeWords,
  figures,
  formatMagnitude,
  formatNumber,
  isRecord,
};

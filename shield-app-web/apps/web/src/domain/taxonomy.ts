import type { components, EventKind, HazardEvent, HazardType, Severity } from "@shield/api-client";
import { hazardType, SEVERITY_ORDER } from "@shield/api-client";

import { hazardTypeTokens, kindTokens, severityTokens } from "@/design/tokens.gen.ts";

/** Hazard types, kinds, severities and the map's categories (shield-core Taxonomy.swift). */

/** Every type the wire knows, `conflict` included (ADR 27). */
type EventType = components["schemas"]["EventType"];
/** The map's menus (openapi.yaml `Category`). */
type Category = components["schemas"]["Category"];

/** An armed-conflict event (UCDP, ADR 27): its own layer, never a hazard. */
const CONFLICT = "conflict" satisfies EventType;

/** Every type but `conflict`, in the apps' order: what the hazard filters offer and "all types" means. */
const HAZARD_TYPES: readonly HazardType[] = [
  "earthquake",
  "wildfire",
  "flood",
  "storm",
  "tropical_cyclone",
  "volcano",
  "drought",
  "heat",
  "cold",
  "heavy_rain",
  "wind",
  "snow_ice",
  "tsunami",
  "other",
];

/** The menus in menu order, each with its types (openapi.yaml `Category`; never invented here). */
const CATEGORY_TYPES: Readonly<Record<Category, readonly EventType[]>> = {
  quakes: ["earthquake", "tsunami"],
  weather: ["storm", "tropical_cyclone", "heavy_rain", "wind", "snow_ice", "heat", "cold", "drought"],
  floods: ["flood"],
  fires: ["wildfire"],
  volcanoes: ["volcano"],
  conflicts: ["conflict"],
  other: ["other"],
};

const CATEGORIES: readonly Category[] = ["quakes", "weather", "floods", "fires", "volcanoes", "conflicts", "other"];

/** For filter words: "Warnings only", "Observed + forecasts". FIRMS is "heat detections", never fire. */
const KIND_PLURALS: Readonly<Record<EventKind, string>> = {
  observed: "observed",
  warning: "warnings",
  forecast: "forecasts",
  detection: "heat detections",
};

/** An open wire `type` on the closed set: `conflict` kept, anything unknown `other`. */
function eventType(wire: string): EventType {
  return wire === CONFLICT ? CONFLICT : hazardType(wire);
}

/** "Earthquake", "Tropical cyclone", "Armed conflict"; an unknown type is "Other hazard". */
function typeLabel(wire: string): string {
  const type = eventType(wire);
  return hazardTypeTokens[type].label;
}

/** The menu a wire `type` belongs to; an unknown type is `other`. */
function categoryOf(wire: string): Category {
  const type = eventType(wire);
  return CATEGORIES.find((category) => CATEGORY_TYPES[category].includes(type)) ?? "other";
}

/** The event's menu: the server's `category` when this client knows it, else derived from `type`. */
function eventCategory(event: Pick<HazardEvent, "type" | "category">): Category {
  if (event.category === undefined) {
    return categoryOf(event.type);
  }
  return CATEGORIES.find((category) => category === event.category) ?? "other";
}

/** 0 for minor up to 3 for extreme. */
function severityRank(severity: Severity): number {
  return SEVERITY_ORDER.indexOf(severity);
}

/** Negative when `left` is less severe than `right`, as a sort comparator. */
function compareSeverity(left: Severity, right: Severity): number {
  return severityRank(left) - severityRank(right);
}

/** True when `severity` is `minimum` or worse. */
function isAtLeast(severity: Severity, minimum: Severity): boolean {
  return compareSeverity(severity, minimum) >= 0;
}

/** "Minor" … "Extreme". */
function severityLabel(severity: Severity): string {
  return severityTokens[severity].label;
}

/** The fixed kind words (DESIGN.md §2): "Observed", "Official warning", "Forecast", "Satellite heat detection". */
function kindLabel(kind: EventKind): string {
  return kindTokens[kind].label;
}

/** "observed", "warnings", "forecasts", "heat detections". */
function kindPluralLabel(kind: EventKind): string {
  return KIND_PLURALS[kind];
}

export type { Category, EventType };
export {
  CATEGORIES,
  CATEGORY_TYPES,
  categoryOf,
  compareSeverity,
  CONFLICT,
  eventCategory,
  eventType,
  HAZARD_TYPES,
  isAtLeast,
  kindLabel,
  kindPluralLabel,
  severityLabel,
  severityRank,
  typeLabel,
};

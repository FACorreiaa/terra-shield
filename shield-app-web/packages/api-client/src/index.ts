import type { Client } from "openapi-fetch";
import createClient from "openapi-fetch";

import type { components, paths } from "./schema.gen.ts";

type Schemas = components["schemas"];
type HazardEvent = Schemas["Event"];
type EventList = Schemas["EventList"];
type EventCluster = Schemas["Cluster"];
type Source = Schemas["Source"];
type Severity = Schemas["Severity"];
type EventKind = Schemas["EventKind"];
/**
 * Closed set the UI draws glyphs for. The wire type is open (ADR 18): unknown values are `other`.
 * `conflict` (ADR 27) has no glyph yet, so it renders as `other` until design adds one.
 */
type HazardType = Exclude<Schemas["EventType"], "conflict">;
type ShieldClient = Client<paths>;

const KNOWN_TYPES: readonly HazardType[] = [
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

/** Map an open wire `type` onto the closed set, so a type added server-side renders as `other`. */
function hazardType(value: string): HazardType {
  return KNOWN_TYPES.find((known) => known === value) ?? "other";
}

const SEVERITY_ORDER: readonly Severity[] = ["minor", "moderate", "severe", "extreme"];

/**
 * A client for the Shield API. `baseUrl` is empty in the browser, where the dev server and the
 * production web server both route `/v1` to the API.
 */
function createShieldClient(baseUrl = "", fetchImpl?: typeof globalThis.fetch): ShieldClient {
  return createClient<paths>({ baseUrl, ...(fetchImpl ? { fetch: fetchImpl } : {}) });
}

export type {
  components,
  EventCluster,
  EventKind,
  EventList,
  HazardEvent,
  HazardType,
  paths,
  Severity,
  ShieldClient,
  Source,
};
export { createShieldClient, hazardType, SEVERITY_ORDER };

import type { Map as MapLibreMap } from "maplibre-gl";

import { alphaTokens, hazardTypeTokens } from "@/design/tokens.gen.ts";
import type { HazardTypeKey, SeverityKey } from "@/design/tokens.gen.ts";

import { glyphMarkup } from "./glyphMarkup.ts";
import type { MarkerKind, MarkerState } from "./markerArt.ts";
import { markerImageName, markerSvg } from "./markerArt.ts";

/** Markers are drawn at twice their size so they stay sharp on high-density screens. */
const PIXEL_RATIO = 2;

interface MarkerRequest {
  type: HazardTypeKey;
  severity: SeverityKey;
  kind: MarkerKind;
  state: MarkerState;
  /** A conflict placed only roughly: drawn with a dashed ring. */
  approximate?: boolean;
}

/** In-flight draws per map, so two views asking for one marker draw it once. */
const inFlight = new WeakMap<MapLibreMap, Map<string, Promise<void>>>();

/** The marker as an image URL, drawn at `pixelRatio`: the map's symbols and DOM copies (an arrival, a toast) alike. */
function markerUrl(request: MarkerRequest, pixelRatio: number = PIXEL_RATIO): string {
  const glyph = glyphMarkup(hazardTypeTokens[request.type].icon);
  const svg = markerSvg({ ...request, glyph }, alphaTokens.stale, pixelRatio);
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

async function draw(map: MapLibreMap, name: string, request: MarkerRequest): Promise<void> {
  const image = new Image();
  image.src = markerUrl(request);
  await image.decode();
  if (!map.hasImage(name)) {
    map.addImage(name, image, { pixelRatio: PIXEL_RATIO });
  }
}

/**
 * Makes sure every marker image the data needs is registered, drawing only the missing ones
 * (type × severity × kind × state is 540 combinations; a view needs a handful). Resolves once all
 * are added, so the caller can set the data after and no marker renders blank.
 */
async function ensureMarkerImages(map: MapLibreMap, requests: readonly MarkerRequest[]): Promise<void> {
  const loading = inFlight.get(map) ?? new Map<string, Promise<void>>();
  inFlight.set(map, loading);
  const missing = requests
    .map((request) => ({ request, name: markerImageName(request.type, request) }))
    .filter(({ name }) => !map.hasImage(name));
  await Promise.all(
    missing.map(async ({ request, name }) => {
      const existing = loading.get(name);
      if (existing !== undefined) {
        await existing;
        return;
      }
      const task = draw(map, name, request);
      loading.set(name, task);
      try {
        await task;
      } finally {
        loading.delete(name);
      }
    }),
  );
}

export type { MarkerRequest };
export { ensureMarkerImages, markerUrl };

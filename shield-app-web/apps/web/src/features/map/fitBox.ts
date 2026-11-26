import type { Map as MapLibreMap } from "maplibre-gl";

import type { GeoBox } from "@/domain/geo.ts";

import { insetsPlus } from "./mapInsets.ts";

const FULL_TURN = 360;
/** A box is never framed narrower than this, in degrees, so one event is not a zoom to a street (HazardMapView.swift). */
const MIN_SPAN_DEG = 0.5;
const HALF = 2;

interface FitOptions {
  /** Pixels kept clear around the box, inside the chrome's insets. */
  breathingRoom: number;
  maxZoom: number;
}

/** `[low, high]`, widened about its middle to at least the minimum span. */
function widened(low: number, high: number): [number, number] {
  if (high - low >= MIN_SPAN_DEG) {
    return [low, high];
  }
  const middle = (low + high) / HALF;
  return [middle - MIN_SPAN_DEG / HALF, middle + MIN_SPAN_DEG / HALF];
}

/** Frames `box` in the visible part of the map; a box across ±180° is unwrapped so the camera takes the short way. */
function fitBox(map: Pick<MapLibreMap, "fitBounds">, box: GeoBox, { breathingRoom, maxZoom }: FitOptions): void {
  const [west, east] = widened(box.west, box.east < box.west ? box.east + FULL_TURN : box.east);
  const [south, north] = widened(box.south, box.north);
  map.fitBounds([west, south, east, north], { padding: insetsPlus(breathingRoom), maxZoom });
}

export type { FitOptions };
export { fitBox };

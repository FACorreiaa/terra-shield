import type { ErrorEvent as MapErrorEvent } from "maplibre-gl";
import { AttributionControl, Map as MapLibreMap } from "maplibre-gl";
import type { RefObject } from "react";
import { useEffect, useEffectEvent, useState } from "react";

import type { View } from "@/domain/viewport.ts";
import { viewOf } from "@/domain/viewport.ts";

import { basemapStyle } from "./basemapStyle.ts";
import type { Projection } from "./basemapStyle.ts";
import { fallbackStyle } from "./fallbackStyle.ts";
import { registerBundledWorker } from "./maplibreWorker.ts";

registerBundledWorker();

/** First view: the Atlantic-centred world, Africa and Europe in the middle. */
const INITIAL = { longitude: 10, latitude: 25, zoom: 2 } as const;
/** A pan settles before the view is reported, so a drag does not fire a query per frame. */
const VIEW_DEBOUNCE_MS = 250;
const TILE_SOURCE = "openmaptiles";
/** One failed tile is a blip; this many means the tile host is unreachable. */
const TILE_FAILURES_BEFORE_FALLBACK = 3;
const FALLBACK_ATTRIBUTION = "Natural Earth";

interface HazardMapState {
  map: MapLibreMap | undefined;
  /** Bumps each time a style finishes loading, so layers and images are added again. */
  styleEpoch: number;
  /** True once the tile basemap failed and the bundled Natural Earth style replaced it. */
  fellBack: boolean;
}

interface HazardMapOptions {
  projection: Projection;
  onViewChange: (view: View) => void;
}

function currentView(map: MapLibreMap): View {
  const bounds = map.getBounds();
  return viewOf(
    { west: bounds.getWest(), south: bounds.getSouth(), east: bounds.getEast(), north: bounds.getNorth() },
    map.getZoom(),
  );
}

/** `metadata` when the TileJSON failed (nothing can draw), `tile` for one tile, else unrelated. */
function tileFailure(event: MapErrorEvent): "metadata" | "tile" | undefined {
  if (!("sourceId" in event) || event.sourceId !== TILE_SOURCE) {
    return undefined;
  }
  return "tile" in event ? "tile" : "metadata";
}

/**
 * Swaps to the bundled Natural Earth style once when OpenFreeMap cannot load (offline, blocked,
 * down). Calls `onFallback` before the swap so the next `style.load` is known to be the fallback.
 */
function watchTileFallback(map: MapLibreMap, onFallback: () => void): void {
  let attribution = new AttributionControl({ compact: true });
  map.addControl(attribution, "bottom-right");
  let fellBack = false;
  let tileFailures = 0;
  map.on("error", (event) => {
    const failure = tileFailure(event);
    tileFailures += failure === "tile" ? 1 : 0;
    const blip = failure === "tile" && tileFailures < TILE_FAILURES_BEFORE_FALLBACK;
    if (fellBack || failure === undefined || blip) {
      return;
    }
    fellBack = true;
    onFallback();
    map.setStyle(fallbackStyle(map.getProjection().type === "mercator" ? "mercator" : "globe"));
    map.removeControl(attribution);
    attribution = new AttributionControl({ compact: true, customAttribution: FALLBACK_ATTRIBUTION });
    map.addControl(attribution, "bottom-right");
  });
}

/** Reports the view on load and after each move settles; returns the cleanup. */
function watchView(map: MapLibreMap, report: (view: View) => void): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined = undefined;
  map.on("load", () => {
    report(currentView(map));
  });
  map.on("moveend", () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      report(currentView(map));
    }, VIEW_DEBOUNCE_MS);
  });
  return () => {
    clearTimeout(timer);
  };
}

function createMap(element: HTMLElement, projection: Projection): MapLibreMap {
  const map = new MapLibreMap({
    container: element,
    style: basemapStyle(projection),
    center: [INITIAL.longitude, INITIAL.latitude],
    zoom: INITIAL.zoom,
    attributionControl: false,
    dragRotate: false,
    pitchWithRotate: false,
  });
  map.touchZoomRotate.disableRotation();
  return map;
}

/**
 * Creates the map once. The basemap is OpenFreeMap styled from tokens, falling back to Natural
 * Earth; the hazard layers come back on whichever style loads (DESIGN.md §9).
 */
function useHazardMap(container: RefObject<HTMLDivElement | null>, options: HazardMapOptions): HazardMapState {
  const [state, setState] = useState<HazardMapState>({ map: undefined, styleEpoch: 0, fellBack: false });
  const viewChanged = useEffectEvent(options.onViewChange);
  const initialProjection = useEffectEvent(() => options.projection);

  useEffect(() => {
    const element = container.current;
    if (element === null) {
      return undefined;
    }
    const map = createMap(element, initialProjection());
    let fellBack = false;
    map.on("style.load", () => {
      setState((previous) => ({ map, styleEpoch: previous.styleEpoch + 1, fellBack }));
    });
    watchTileFallback(map, () => {
      fellBack = true;
    });
    const stopView = watchView(map, (view) => {
      viewChanged(view);
    });
    return () => {
      stopView();
      setState({ map: undefined, styleEpoch: 0, fellBack: false });
      map.remove();
    };
  }, [container]);

  const { map, styleEpoch } = state;
  useEffect(() => {
    if (map !== undefined && styleEpoch > 0) {
      map.setProjection({ type: options.projection });
    }
  }, [map, styleEpoch, options.projection]);

  return state;
}

export type { HazardMapState };
export { useHazardMap };

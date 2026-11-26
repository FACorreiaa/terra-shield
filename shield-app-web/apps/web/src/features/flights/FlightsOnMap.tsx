import type { Aircraft } from "@shield/api-client";
import type { ExpressionSpecification, GeoJSONSource, Map as MapLibreMap, Point, PointLike } from "maplibre-gl";
import { useEffect, useEffectEvent } from "react";

import { EVENTS_LAYER, SELECTION_LAYER } from "@/features/map/eventLayer.ts";

import {
  AIRCRAFT_LAYER,
  AIRCRAFT_SELECTION_LAYER,
  AIRCRAFT_SOURCE,
  aircraftFeatures,
  aircraftImages,
  aircraftLayer,
  aircraftSelectionLayer,
  PIXEL_RATIO,
} from "./flightLayer.ts";

interface FlightsOnMapProps {
  map: MapLibreMap;
  /** Bumped each time the style (re)loads: the layers are added again. */
  styleEpoch: number;
  aircraft: readonly Aircraft[] | undefined;
  selectedHex: string | undefined;
  onSelect: (hex: string) => void;
}

async function addImages(map: MapLibreMap): Promise<void> {
  await Promise.all(
    Object.entries(aircraftImages()).map(async ([name, markup]) => {
      if (map.hasImage(name)) {
        return;
      }
      const image = new Image();
      image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(markup)}`;
      await image.decode();
      if (!map.hasImage(name)) {
        map.addImage(name, image, { pixelRatio: PIXEL_RATIO });
      }
    }),
  );
}

/** The source and layers, under the hazards' selection ring and markers when those are there already. */
function addLayers(map: MapLibreMap): void {
  if (map.getSource(AIRCRAFT_SOURCE) !== undefined) {
    return;
  }
  map.addSource(AIRCRAFT_SOURCE, { type: "geojson", data: aircraftFeatures(undefined) });
  const below = map.getLayer(SELECTION_LAYER) === undefined ? undefined : SELECTION_LAYER;
  map.addLayer(aircraftSelectionLayer(undefined), below);
  map.addLayer(aircraftLayer(), below);
}

/** Half the 44 px hit target around a click: the drawn aircraft is 28 px, as the apps'. */
const REACH = 22;

/** The aircraft nearest a click, within reach; none when a hazard marker is under it (the app's subject wins). */
function aircraftAt(map: MapLibreMap, point: Point): string | undefined {
  if (map.getLayer(AIRCRAFT_LAYER) === undefined) {
    return undefined;
  }
  const box: [PointLike, PointLike] = [
    [point.x - REACH, point.y - REACH],
    [point.x + REACH, point.y + REACH],
  ];
  const near = map.queryRenderedFeatures(box, { layers: [AIRCRAFT_LAYER] });
  const hazards =
    map.getLayer(EVENTS_LAYER) === undefined
      ? []
      : map.queryRenderedFeatures([point.x, point.y], { layers: [EVENTS_LAYER] });
  const distance = (feature: (typeof near)[number]): number => {
    if (feature.geometry.type !== "Point") {
      return Number.POSITIVE_INFINITY;
    }
    const [lng = 0, lat = 0] = feature.geometry.coordinates;
    return map.project([lng, lat]).dist(point);
  };
  const [nearest] = near.toSorted((first, second) => distance(first) - distance(second));
  const hex: unknown = nearest?.properties["hex"];
  return hazards.length === 0 && typeof hex === "string" ? hex : undefined;
}

/** Selection by pointer: clicks on the canvas only, not on the cluster buttons over it. */
function useAircraftInput(map: MapLibreMap, onSelect: (hex: string) => void): void {
  const selected = useEffectEvent(onSelect);
  useEffect(() => {
    const canvas = map.getCanvas();
    const click = map.on("click", (event) => {
      const hex = event.originalEvent.target === canvas ? aircraftAt(map, event.point) : undefined;
      if (hex !== undefined) {
        selected(hex);
      }
    });
    const enter = map.on("mouseenter", AIRCRAFT_LAYER, () => {
      canvas.style.cursor = "pointer";
    });
    const leave = map.on("mouseleave", AIRCRAFT_LAYER, () => {
      canvas.style.cursor = "";
    });
    return () => {
      click.unsubscribe();
      enter.unsubscribe();
      leave.unsubscribe();
    };
  }, [map]);
}

/** The layers (added once per style), the selection and the positions. */
async function draw(
  map: MapLibreMap,
  aircraft: readonly Aircraft[] | undefined,
  selectedHex: string | undefined,
): Promise<void> {
  addLayers(map);
  const isSelected: ExpressionSpecification = ["==", ["get", "hex"], selectedHex ?? ""];
  map.setFilter(AIRCRAFT_SELECTION_LAYER, isSelected);
  // The selected aircraft is placed first, so crowding never hides it over its own disc.
  map.setLayoutProperty(AIRCRAFT_LAYER, "symbol-sort-key", ["case", isSelected, 0, 1]);
  await map.getSource<GeoJSONSource>(AIRCRAFT_SOURCE)?.setData(aircraftFeatures(aircraft));
}

/**
 * The live flights layer on the canvas: the plane or dot per aircraft, and an `accent` disc under
 * the selected one. Renders nothing itself; the map draws it.
 */
function FlightsOnMap({ map, styleEpoch, aircraft, selectedHex, onSelect }: FlightsOnMapProps): undefined {
  useAircraftInput(map, onSelect);
  useEffect(() => {
    if (styleEpoch === 0) {
      return undefined;
    }
    let current = true;
    const apply = async (): Promise<void> => {
      try {
        await addImages(map);
        if (current) {
          await draw(map, aircraft, selectedHex);
        }
      } catch {
        // A style swapped mid-way (the basemap's fallback) throws; its new epoch draws again.
      }
    };
    void apply();
    return () => {
      current = false;
    };
  }, [map, styleEpoch, aircraft, selectedHex]);
  return undefined;
}

export { FlightsOnMap };

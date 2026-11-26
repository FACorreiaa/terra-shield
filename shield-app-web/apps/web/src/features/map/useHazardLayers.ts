import type { HazardEvent } from "@shield/api-client";
import type { FeatureCollection } from "geojson";
import type { GeoJSONSource, Map as MapLibreMap } from "maplibre-gl";
import { useEffect, useEffectEvent } from "react";

import type { EventShape } from "@/domain/geo.ts";

import { AREA_SOURCE, areaFeatures, areaLayers } from "./areaLayer.ts";
import {
  EVENTS_LAYER,
  EVENTS_SOURCE,
  eventFeatures,
  eventLayer,
  markerRequests,
  SELECTION_LAYER,
  selectionLayer,
} from "./eventLayer.ts";
import { ensureMarkerImages } from "./markerImages.ts";

const NO_FEATURES: FeatureCollection = { type: "FeatureCollection", features: [] };

interface HazardLayerData {
  events: readonly HazardEvent[];
  staleSources: ReadonlySet<string>;
  selectedId: string | undefined;
  selectedShape: EventShape | undefined;
}

/** Adds the hazard sources and layers to a freshly loaded style (DESIGN.md §9 order). */
function addHazardLayers(map: MapLibreMap): void {
  if (map.getSource(EVENTS_SOURCE) !== undefined) {
    return;
  }
  map.addSource(AREA_SOURCE, { type: "geojson", data: NO_FEATURES });
  // `promoteId`: features keyed by event id, so a live arrival's symbol can wait (feature-state) while its copy animates.
  map.addSource(EVENTS_SOURCE, { type: "geojson", data: NO_FEATURES, promoteId: "id" });
  for (const layer of areaLayers()) {
    map.addLayer(layer);
  }
  map.addLayer(selectionLayer(undefined));
  map.addLayer(eventLayer());
}

/** Pointer feedback and selection on the marker layer; registered once per map. */
function useMarkerInput(map: MapLibreMap | undefined, onSelect: (id: string) => void): void {
  const selected = useEffectEvent(onSelect);
  useEffect(() => {
    if (map === undefined) {
      return undefined;
    }
    const canvas = map.getCanvas();
    const click = map.on("click", EVENTS_LAYER, (event) => {
      const id: unknown = event.features?.[0]?.properties["id"];
      if (typeof id === "string") {
        selected(id);
      }
    });
    const enter = map.on("mouseenter", EVENTS_LAYER, () => {
      canvas.style.cursor = "pointer";
    });
    const leave = map.on("mouseleave", EVENTS_LAYER, () => {
      canvas.style.cursor = "";
    });
    return () => {
      click.unsubscribe();
      enter.unsubscribe();
      leave.unsubscribe();
    };
  }, [map]);
}

/** Keeps the marker source in step with the events, drawing any missing marker images first. */
function useEventData(map: MapLibreMap | undefined, styleEpoch: number, data: HazardLayerData): void {
  const { events, staleSources } = data;
  useEffect(() => {
    if (map === undefined || styleEpoch === 0) {
      return undefined;
    }
    addHazardLayers(map);
    let current = true;
    const apply = async (): Promise<void> => {
      await ensureMarkerImages(map, markerRequests(events, staleSources));
      if (current) {
        await map.getSource<GeoJSONSource>(EVENTS_SOURCE)?.setData(eventFeatures(events, staleSources));
      }
    };
    void apply();
    return () => {
      current = false;
    };
  }, [map, styleEpoch, events, staleSources]);
}

/** The selection ring and the selected event's area. */
function useSelection(map: MapLibreMap | undefined, styleEpoch: number, data: HazardLayerData): void {
  const { selectedId, selectedShape, events, staleSources } = data;
  const event = events.find((candidate) => candidate.id === selectedId);
  useEffect(() => {
    if (map === undefined || styleEpoch === 0) {
      return;
    }
    addHazardLayers(map);
    map.setFilter(SELECTION_LAYER, ["==", ["get", "id"], selectedId ?? ""]);
    const faded = event !== undefined && (event.status === "resolved" || staleSources.has(event.source));
    const selected = event && selectedShape && { event, shape: selectedShape, faded };
    void map.getSource<GeoJSONSource>(AREA_SOURCE)?.setData(areaFeatures(selected));
  }, [map, styleEpoch, selectedId, selectedShape, event, staleSources]);
}

/** Every canvas hazard layer: areas, selection ring, point markers, and their input. */
function useHazardLayers(
  map: MapLibreMap | undefined,
  styleEpoch: number,
  data: HazardLayerData & { onSelect: (id: string) => void },
): void {
  useMarkerInput(map, data.onSelect);
  useEventData(map, styleEpoch, data);
  useSelection(map, styleEpoch, data);
}

export type { HazardLayerData };
export { useHazardLayers };

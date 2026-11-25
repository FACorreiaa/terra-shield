import type { HazardEvent } from "@shield/api-client";
import { Map as MapIcon } from "lucide-react";
import type { GeoJSONSource } from "maplibre-gl";
import { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement, RefObject } from "react";
import { useEffect, useRef, useState } from "react";

import type { EventShape } from "@/domain/geo.ts";
import { frame } from "@/domain/geo.ts";
import { AREA_SOURCE, areaFeatures, areaLayers } from "@/features/map/areaLayer.ts";
import { basemapStyle } from "@/features/map/basemapStyle.ts";
import { MapMarker } from "@/features/map/MapMarker.tsx";

import { EventGlyph } from "./EventGlyph.tsx";

const FULL_TURN = 360;
const PADDING_PX = 16;
const NO_AREA = areaFeatures(undefined);

interface EventMapSnippetProps {
  event: HazardEvent;
  /** The detail's area once loaded; the centroid until then. */
  shape: EventShape | undefined;
  faded: boolean;
  onShowOnMap: () => void;
}

/** One still map, created once (the detail keys it by event), given the area when it arrives, removed with it. */
function useSnippetMap(container: RefObject<HTMLDivElement | null>): MapLibreMap | undefined {
  const [map, setMap] = useState<MapLibreMap>();
  useEffect(() => {
    const element = container.current;
    if (element === null) {
      return undefined;
    }
    const created = new MapLibreMap({
      container: element,
      style: basemapStyle("mercator"),
      interactive: false,
      // Credited below the snippet: controls inside it would sit under its button.
      attributionControl: false,
    });
    created.on("load", () => {
      created.addSource(AREA_SOURCE, { type: "geojson", data: NO_AREA });
      for (const layer of areaLayers()) {
        created.addLayer(layer);
      }
      setMap(created);
    });
    return () => {
      setMap(undefined);
      created.remove();
    };
  }, [container]);
  return map;
}

/**
 * The event's geometry on a small, still map (DESIGN.md §10.3 item 5): its area drawn by kind
 * (§5) with the marker on top. Not interactive; pressing it shows the event on the main map.
 */
function EventMapSnippet({ event, shape, faded, onShowOnMap }: EventMapSnippetProps): ReactElement {
  const container = useRef<HTMLDivElement>(null);
  const map = useSnippetMap(container);
  useEffect(() => {
    if (map === undefined) {
      return;
    }
    const box = frame(shape ?? { kind: "point", point: event.centroid }, event.centroid);
    const east = box.east < box.west ? box.east + FULL_TURN : box.east;
    map.fitBounds([box.west, box.south, east, box.north], { padding: PADDING_PX, animate: false });
    const area = shape === undefined ? undefined : { event, shape, faded };
    void map.getSource<GeoJSONSource>(AREA_SOURCE)?.setData(areaFeatures(area));
  }, [map, shape, faded, event]);
  const what = shape !== undefined && shape.kind !== "point" ? "area" : "location";
  return (
    <div className="relative h-40 w-full overflow-hidden rounded-md border border-hairline bg-canvas">
      {/* Sized, not inset: MapLibre makes its container `position: relative`. */}
      <div ref={container} aria-hidden className="size-full" />
      {map === undefined ? undefined : (
        <MapMarker map={map} longitude={event.centroid.longitude} latitude={event.centroid.latitude}>
          <EventGlyph event={event} state={faded ? "stale" : "live"} className="size-7" />
        </MapMarker>
      )}
      {/* A larger target for the pointer; keyboards and screen readers use "Show on map" below. */}
      <button
        type="button"
        tabIndex={-1}
        aria-hidden
        onClick={onShowOnMap}
        title={`Show the event’s ${what} on the main map`}
        className="absolute inset-0 rounded-md focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent"
      />
    </div>
  );
}

/** The basemap's credit, which the snippet has no room for (DESIGN.md §9). */
function MapCredit(): ReactElement {
  return (
    <p className="text-micro tracking-normal text-faint normal-case">
      <a href="https://openfreemap.org" target="_blank" rel="noopener noreferrer" className="hover:text-muted">
        OpenFreeMap
      </a>{" "}
      <a href="https://www.openmaptiles.org/" target="_blank" rel="noopener noreferrer" className="hover:text-muted">
        © OpenMapTiles
      </a>{" "}
      Data from{" "}
      <a
        href="https://www.openstreetmap.org/copyright"
        target="_blank"
        rel="noopener noreferrer"
        className="hover:text-muted"
      >
        OpenStreetMap
      </a>
    </p>
  );
}

/** The snippet and its plain "Show on map" link, as in the apps. */
function EventMapBlock(props: EventMapSnippetProps): ReactElement {
  return (
    <div className="flex flex-col gap-2">
      <EventMapSnippet
        key={props.event.id}
        event={props.event}
        shape={props.shape}
        faded={props.faded}
        onShowOnMap={props.onShowOnMap}
      />
      <MapCredit />
      <button
        type="button"
        onClick={props.onShowOnMap}
        className="inline-flex min-h-11 items-center gap-2 self-start rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
      >
        <MapIcon className="size-4" aria-hidden />
        Show on map
      </button>
    </div>
  );
}

export { EventMapBlock };

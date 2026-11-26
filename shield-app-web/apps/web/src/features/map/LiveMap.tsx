import { Crosshair, Plane, Thermometer } from "lucide-react";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement, ReactNode } from "react";
import { useCallback, useMemo, useState } from "react";

import { toggleShowsConflicts, toggleShowsDetections, useSavedFilters } from "@/features/filters/filterPrefs.ts";
import { useFlightsLayer } from "@/features/flights/flightsContext.ts";
import { FlightsOnMap } from "@/features/flights/FlightsOnMap.tsx";

import type { HazardMapProps } from "./HazardMap.tsx";
import { HazardMap } from "./HazardMap.tsx";
import type { LayerSwitch } from "./LayerToggle.tsx";
import { MapControls } from "./MapControls.tsx";
import { setProjection, useMapPrefs } from "./mapPrefs.ts";

interface LiveMapProps extends Omit<HazardMapProps, "projection" | "layers"> {
  /** The selected aircraft (live flights), and selecting one by pointer. */
  aircraftHex: string | undefined;
  onSelectAircraft: (hex: string) => void;
}

const ACCENT_PRESSED = "bg-accent/14 text-accent hover:bg-accent/20";
const DETECTION_PRESSED = "bg-kind-detection/14 text-kind-detection hover:bg-kind-detection/20";

/** The layers' switches, top to bottom as in the apps' control stack: flights, conflicts, heat. */
function useLayerSwitches(): readonly LayerSwitch[] {
  const { showsDetections, showsConflicts } = useSavedFilters();
  const { shown: showsFlights, toggle: toggleFlights } = useFlightsLayer();
  return useMemo(
    () => [
      // Live flights (ADR 28): off at every load, never saved.
      {
        id: "flights",
        label: "Live flights",
        icon: Plane,
        pressed: showsFlights,
        pressedClass: ACCENT_PRESSED,
        onToggle: toggleFlights,
      },
      // Armed conflicts (ADR 27): a layer beside hazards, off by default.
      {
        id: "conflicts",
        label: "Armed conflicts",
        icon: Crosshair,
        pressed: showsConflicts,
        pressedClass: ACCENT_PRESSED,
        onToggle: toggleShowsConflicts,
      },
      // Off by default (DESIGN.md §5). The word is "heat detections", never "fires".
      {
        id: "detections",
        label: "Satellite heat detections",
        icon: Thermometer,
        pressed: showsDetections,
        pressedClass: DETECTION_PRESSED,
        onToggle: toggleShowsDetections,
      },
    ],
    [showsFlights, toggleFlights, showsConflicts, showsDetections],
  );
}

/** The live flights' canvas layers, for the hazard map to draw once it has a map. */
function useFlightLayers(
  aircraftHex: string | undefined,
  onSelectAircraft: (hex: string) => void,
): (map: MapLibreMap, styleEpoch: number) => ReactNode {
  const { list } = useFlightsLayer();
  return useCallback(
    (live: MapLibreMap, styleEpoch: number): ReactNode => (
      <FlightsOnMap
        map={live}
        styleEpoch={styleEpoch}
        aircraft={list?.aircraft}
        selectedHex={aircraftHex}
        onSelect={onSelectAircraft}
      />
    ),
    [list, aircraftHex, onSelectAircraft],
  );
}

/**
 * The hazard map with its floating controls, in the projection the person last chose. The controls
 * sit inside the shell's insets (`--map-inset-top`, `--map-inset-right`), clear of its chrome.
 */
function LiveMap(props: LiveMapProps): ReactElement {
  const { aircraftHex, onSelectAircraft, onMap, onViewChange, onSelect, ...shown } = props;
  const [map, setMap] = useState<MapLibreMap>();
  const mapChanged = useCallback(
    (next: MapLibreMap | undefined) => {
      setMap(next);
      onMap?.(next);
    },
    [onMap],
  );
  const { projection } = useMapPrefs();
  const layers = useLayerSwitches();
  const flightLayers = useFlightLayers(aircraftHex, onSelectAircraft);
  return (
    <>
      <HazardMap
        events={shown.events}
        clusters={shown.clusters}
        conflictClusters={shown.conflictClusters}
        staleSources={shown.staleSources}
        selectedId={shown.selectedId}
        selectedShape={shown.selectedShape}
        projection={projection}
        onViewChange={onViewChange}
        onSelect={onSelect}
        onMap={mapChanged}
        arrivals={shown.arrivals}
        arrival={shown.arrival}
        detections={shown.detections}
        layers={flightLayers}
      />
      <div className="absolute top-[calc(var(--map-inset-top,0px)+0.75rem)] right-[calc(var(--map-inset-right,0px)+0.75rem)] z-10">
        <MapControls map={map} projection={projection} onProjectionChange={setProjection} layers={layers} />
      </div>
    </>
  );
}

export { LiveMap };

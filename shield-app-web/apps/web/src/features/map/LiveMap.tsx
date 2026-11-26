import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback, useState } from "react";

import { toggleShowsConflicts, toggleShowsDetections, useSavedFilters } from "@/features/filters/filterPrefs.ts";

import type { HazardMapProps } from "./HazardMap.tsx";
import { HazardMap } from "./HazardMap.tsx";
import { MapControls } from "./MapControls.tsx";
import { setProjection, useMapPrefs } from "./mapPrefs.ts";

type LiveMapProps = Omit<HazardMapProps, "projection">;

/**
 * The hazard map with its floating controls, in the projection the person last chose. The controls
 * sit inside the shell's insets (`--map-inset-top`, `--map-inset-right`), clear of its chrome.
 */
function LiveMap(props: LiveMapProps): ReactElement {
  const {
    events,
    clusters,
    conflictClusters,
    staleSources,
    selectedId,
    selectedShape,
    onViewChange,
    onSelect,
    onMap,
    arrivals,
    arrival,
    detections,
  } = props;
  const [map, setMap] = useState<MapLibreMap>();
  const mapChanged = useCallback(
    (next: MapLibreMap | undefined) => {
      setMap(next);
      onMap?.(next);
    },
    [onMap],
  );
  const { projection } = useMapPrefs();
  const { showsDetections, showsConflicts } = useSavedFilters();
  return (
    <>
      <HazardMap
        events={events}
        clusters={clusters}
        conflictClusters={conflictClusters}
        staleSources={staleSources}
        selectedId={selectedId}
        selectedShape={selectedShape}
        projection={projection}
        onViewChange={onViewChange}
        onSelect={onSelect}
        onMap={mapChanged}
        arrivals={arrivals}
        arrival={arrival}
        detections={detections}
      />
      <div className="absolute top-[calc(var(--map-inset-top,0px)+0.75rem)] right-[calc(var(--map-inset-right,0px)+0.75rem)] z-10">
        <MapControls
          map={map}
          projection={projection}
          showsDetections={showsDetections}
          showsConflicts={showsConflicts}
          onProjectionChange={setProjection}
          onDetectionsToggle={toggleShowsDetections}
          onConflictsToggle={toggleShowsConflicts}
        />
      </div>
    </>
  );
}

export { LiveMap };

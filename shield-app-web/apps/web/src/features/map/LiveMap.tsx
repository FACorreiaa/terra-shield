import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useState } from "react";

import type { HazardMapProps } from "./HazardMap.tsx";
import { HazardMap } from "./HazardMap.tsx";
import { MapControls } from "./MapControls.tsx";
import { setProjection, useMapPrefs } from "./mapPrefs.ts";

type LiveMapProps = Omit<HazardMapProps, "projection" | "onMap">;

/** The hazard map with its floating controls, in the projection the person last chose. */
function LiveMap(props: LiveMapProps): ReactElement {
  const { events, clusters, staleSources, selectedId, selectedShape, onViewChange, onSelect } = props;
  const [map, setMap] = useState<MapLibreMap>();
  const { projection } = useMapPrefs();
  return (
    <>
      <HazardMap
        events={events}
        clusters={clusters}
        staleSources={staleSources}
        selectedId={selectedId}
        selectedShape={selectedShape}
        projection={projection}
        onViewChange={onViewChange}
        onSelect={onSelect}
        onMap={setMap}
      />
      <div className="absolute top-3 right-3 z-10">
        <MapControls map={map} projection={projection} onProjectionChange={setProjection} />
      </div>
    </>
  );
}

export { LiveMap };

import { Earth, Minus, Plus } from "lucide-react";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback } from "react";

import type { Projection } from "./basemapStyle.ts";
import { controlButton } from "./controlStyles.ts";
import { LayersMenu } from "./LayersMenu.tsx";
import { fitWorld } from "./useHazardMap.ts";

interface MapControlsProps {
  /** The live map; the zoom and world buttons wait for it. */
  map: MapLibreMap | undefined;
  projection: Projection;
  onProjectionChange: (projection: Projection) => void;
}

const ICON = "size-4";
const STROKE = 2.25;

function Divider(): ReactElement {
  return <hr className="mx-2 border-hairline-strong" aria-hidden />;
}

/**
 * Floating map controls, top-right of the canvas (DESIGN.md §9): layers, the whole world and
 * zoom ±, one 44 px column on `floating` material, like the apps' control stack. Locate me
 * arrives with the shells (W3).
 */
function MapControls({ map, projection, onProjectionChange }: MapControlsProps): ReactElement {
  const world = useCallback(() => {
    if (map !== undefined) {
      fitWorld(map, projection);
    }
  }, [map, projection]);
  const zoomIn = useCallback(() => {
    map?.zoomIn();
  }, [map]);
  const zoomOut = useCallback(() => {
    map?.zoomOut();
  }, [map]);
  const ready = map !== undefined;

  return (
    <div className="flex w-11 flex-col rounded-md border border-hairline-strong shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none">
      <LayersMenu projection={projection} onProjectionChange={onProjectionChange} />
      <Divider />
      <button
        type="button"
        className={controlButton}
        aria-label="Show the whole world"
        title="Show the whole world"
        disabled={!ready}
        onClick={world}
      >
        <Earth className={ICON} strokeWidth={STROKE} aria-hidden />
      </button>
      <Divider />
      <button
        type="button"
        className={controlButton}
        aria-label="Zoom in"
        title="Zoom in"
        disabled={!ready}
        onClick={zoomIn}
      >
        <Plus className={ICON} strokeWidth={STROKE} aria-hidden />
      </button>
      <button
        type="button"
        className={controlButton}
        aria-label="Zoom out"
        title="Zoom out"
        disabled={!ready}
        onClick={zoomOut}
      >
        <Minus className={ICON} strokeWidth={STROKE} aria-hidden />
      </button>
    </div>
  );
}

export { MapControls };

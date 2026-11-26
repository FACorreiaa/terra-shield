import { Earth, Minus, Plus, Thermometer } from "lucide-react";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback } from "react";

import type { Projection } from "./basemapStyle.ts";
import { controlButton } from "./controlStyles.ts";
import { LayersMenu } from "./LayersMenu.tsx";
import { LayerToggle } from "./LayerToggle.tsx";
import { fitWorld } from "./useHazardMap.ts";

interface MapControlsProps {
  /** The live map; the zoom and world buttons wait for it. */
  map: MapLibreMap | undefined;
  projection: Projection;
  onProjectionChange: (projection: Projection) => void;
  /** The heat layer's switch, shared with the filter sheet and `h`. */
  showsDetections: boolean;
  onDetectionsToggle: () => void;
}

const ICON = "size-4";
const STROKE = 2.25;

function Divider(): ReactElement {
  return <hr className="mx-2 border-hairline-strong" aria-hidden />;
}

/**
 * Floating map controls, top-right of the canvas (DESIGN.md §9): the projection, a switch per
 * layer, the whole world and zoom ±, one 44 px column on `floating` material, like the apps'
 * control stack (layers are one-press toggles there too).
 */
function MapControls(props: MapControlsProps): ReactElement {
  const { map, projection, onProjectionChange, showsDetections, onDetectionsToggle } = props;
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
      {/* Off by default (DESIGN.md §5). The word is "heat detections", never "fires". */}
      <LayerToggle
        label="Satellite heat detections"
        icon={Thermometer}
        pressed={showsDetections}
        pressedClass="bg-kind-detection/14 text-kind-detection hover:bg-kind-detection/20"
        onToggle={onDetectionsToggle}
      />
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

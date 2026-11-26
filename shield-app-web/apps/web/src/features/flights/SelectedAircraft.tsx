import type { ReactElement } from "react";

import { AircraftDetail } from "./AircraftDetail.tsx";
import { useFlightsLayer } from "./flightsContext.ts";

/** The selected aircraft's position report; nothing once it has gone (the layer off, zoomed out). */
function SelectedAircraft({ onClose }: { onClose: () => void }): ReactElement | undefined {
  const { selected, list } = useFlightsLayer();
  return selected === undefined ? undefined : <AircraftDetail aircraft={selected} list={list} onClose={onClose} />;
}

export { SelectedAircraft };

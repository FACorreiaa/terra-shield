import type { Aircraft } from "@shield/api-client";
import { createContext, use } from "react";

import type { Flights } from "./useFlights.ts";

/** The live flights layer for the whole app: its own context, so its minute updates re-render only what shows flights. */
interface FlightsLayer extends Flights {
  toggle: () => void;
  /** The selected aircraft: this answer's copy, else the last one seen while it is out of view. */
  selected: Aircraft | undefined;
}

const FlightsContext = createContext<FlightsLayer | undefined>(undefined);

function useFlightsLayer(): FlightsLayer {
  const value = use(FlightsContext);
  if (value === undefined) {
    throw new Error("useFlightsLayer needs a FlightsProvider above it");
  }
  return value;
}

export type { FlightsLayer };
export { FlightsContext, useFlightsLayer };

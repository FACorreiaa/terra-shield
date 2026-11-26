import type { Aircraft } from "@shield/api-client";
import type { ReactElement, ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import type { View } from "@/domain/viewport.ts";

import type { FlightsLayer } from "./flightsContext.ts";
import { FlightsContext } from "./flightsContext.ts";
import { useFlights } from "./useFlights.ts";

interface FlightsProviderProps {
  view: View;
  aircraftHex: string | undefined;
  selectAircraft: (hex: string | undefined) => void;
  children: ReactNode;
}

/** The selected aircraft as last seen, kept while it is out of the answer (LiveMapModel `chosenAircraft`). */
function useLastSeen(list: readonly Aircraft[] | undefined, hex: string | undefined): Aircraft | undefined {
  const [seen, setSeen] = useState<Aircraft>();
  const current = hex === undefined ? undefined : list?.find((aircraft) => aircraft.hex === hex);
  if (current !== undefined && current !== seen) {
    setSeen(current);
  }
  if (current !== undefined) {
    return current;
  }
  return seen?.hex === hex ? seen : undefined;
}

/**
 * Live flights (ADR 28): off at every load and never saved, so a visit can't start by asking the
 * feed. Turning the layer off or zooming out past what the server serves drops the selected aircraft.
 */
function FlightsProvider({ view, aircraftHex, selectAircraft, children }: FlightsProviderProps): ReactElement {
  const [shown, setShown] = useState(false);
  const flights = useFlights(view, shown);
  const toggle = useCallback(() => {
    setShown((was) => !was);
  }, []);
  const selected = useLastSeen(flights.list?.aircraft, aircraftHex);
  const gone = !shown || flights.needsZoom;
  useEffect(() => {
    if (gone && aircraftHex !== undefined) {
      selectAircraft(undefined);
    }
  }, [gone, aircraftHex, selectAircraft]);
  const value = useMemo<FlightsLayer>(() => ({ ...flights, toggle, selected }), [flights, toggle, selected]);
  return <FlightsContext value={value}>{children}</FlightsContext>;
}

export { FlightsProvider };

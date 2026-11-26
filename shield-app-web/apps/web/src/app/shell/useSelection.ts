import { useCallback, useMemo, useState } from "react";

import type { UrlSelection } from "./useUrlSelection.ts";
import { useUrlSelection } from "./useUrlSelection.ts";

interface Selection extends UrlSelection {
  /** The selected aircraft's address (`hex`), while one is: never with an event. */
  aircraftHex: string | undefined;
  selectAircraft: (hex: string | undefined) => void;
  /** An event or an aircraft is selected. */
  inspecting: boolean;
}

/**
 * One thing selected at a time, an event or an aircraft (LiveMapModel `clearSelections`). The event
 * lives in the address; an aircraft doesn't, since its position is minutes old and a link to it
 * would open on nothing.
 */
function useSelection(): Selection {
  const url = useUrlSelection();
  const { select: selectEvent, choose: chooseEvent } = url;
  const [aircraftHex, setAircraftHex] = useState<string>();
  const select = useCallback(
    (id: string | undefined) => {
      if (id !== undefined) {
        setAircraftHex(undefined);
      }
      selectEvent(id);
    },
    [selectEvent],
  );
  const choose = useCallback<UrlSelection["choose"]>(
    (event) => {
      setAircraftHex(undefined);
      chooseEvent(event);
    },
    [chooseEvent],
  );
  const selectAircraft = useCallback(
    (hex: string | undefined) => {
      if (hex !== undefined) {
        selectEvent(undefined);
      }
      setAircraftHex(hex);
    },
    [selectEvent],
  );
  const { selectedId, chosen } = url;
  // An event that arrives by the address (Back, Forward, a link) also replaces the aircraft.
  const [eventId, setEventId] = useState(selectedId);
  if (selectedId !== eventId) {
    setEventId(selectedId);
    if (selectedId !== undefined && aircraftHex !== undefined) {
      setAircraftHex(undefined);
    }
  }
  return useMemo(
    () => ({
      selectedId,
      chosen,
      select,
      choose,
      aircraftHex,
      selectAircraft,
      inspecting: selectedId !== undefined || aircraftHex !== undefined,
    }),
    [selectedId, chosen, select, choose, aircraftHex, selectAircraft],
  );
}

export type { Selection };
export { useSelection };

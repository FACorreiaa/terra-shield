import type { HazardEvent } from "@shield/api-client";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useCallback, useState } from "react";

interface UrlSelection {
  selectedId: string | undefined;
  select: (id: string | undefined) => void;
  /** The copy of an event chosen outside the map's list (a country's), while it is selected. */
  chosen: HazardEvent | undefined;
  /** Selects an event the map may not show, keeping its copy until the detail loads. */
  choose: (event: HazardEvent) => void;
}

/**
 * The selected event lives in the address (`?event=<id>`) on every section, so a country or a
 * search stays open beside it, as on the Mac, and a reload or a shared link opens it again. Opening
 * one from nothing adds a history step, so Back closes it; switching or closing replaces the step.
 */
function useUrlSelection(): UrlSelection {
  const { event: selectedId } = useSearch({ from: "/_app" });
  const navigate = useNavigate();
  const select = useCallback(
    (id: string | undefined) => {
      if (id === selectedId) {
        return;
      }
      void navigate({
        to: ".",
        search: (previous) => ({ ...previous, event: id }),
        replace: selectedId !== undefined || id === undefined,
      });
    },
    [selectedId, navigate],
  );
  const [chosen, setChosen] = useState<HazardEvent>();
  const choose = useCallback(
    (event: HazardEvent) => {
      setChosen(event);
      select(event.id);
    },
    [select],
  );
  return { selectedId, select, chosen: chosen?.id === selectedId ? chosen : undefined, choose };
}

export { useUrlSelection };

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useCallback } from "react";

interface UrlSelection {
  selectedId: string | undefined;
  select: (id: string | undefined) => void;
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
  return { selectedId, select };
}

export { useUrlSelection };

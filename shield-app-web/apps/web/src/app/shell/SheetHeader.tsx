import type { ReactElement } from "react";
import { use } from "react";

import { CompactStatStrip } from "@/features/monitor/StatStrip.tsx";

import { MapSearchBox } from "./MapSearchBox.tsx";
import { useMapSearch } from "./mapSearchContext.ts";
import { RestingDetent } from "./sheetDetents.ts";
import { useCells } from "./useCells.ts";

/**
 * Always in view, even at peek (DESIGN.md §10 shells): the search field over the compact strip.
 * From half height up, and while searching, the line hides but keeps its height, so the peek never
 * shifts; the full strip takes over in the sheet's body. The section's name is for screen readers:
 * the field and the numbers say it on screen.
 */
function SheetHeader(): ReactElement {
  const detent = use(RestingDetent);
  const { active } = useMapSearch();
  const cells = useCells();
  return (
    <div className="flex flex-col gap-2 px-4 pb-3">
      <h1 className="sr-only">Live events</h1>
      <MapSearchBox cancel />
      <div className={detent === "peek" && !active ? undefined : "invisible"} data-testid="sheet-strip-line">
        <CompactStatStrip cells={cells} />
      </div>
    </div>
  );
}

export { SheetHeader };

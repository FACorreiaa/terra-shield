import type { ReactElement } from "react";
import { use } from "react";

import { CompactStatStrip } from "@/features/monitor/StatStrip.tsx";

import { MapSearchBox } from "./MapSearchBox.tsx";
import { useMapSearch } from "./mapSearchContext.ts";
import { RestingDetent } from "./sheetDetents.ts";
import { SourcesButton, SourcesTitle } from "./SourcesSheetParts.tsx";
import { useCells } from "./useCells.ts";
import type { Page } from "./usePage.ts";
import { usePage } from "./usePage.ts";

/**
 * Always in view, even at peek (DESIGN.md §10 shells): the search field over the compact strip.
 * From half height up, and while searching, the line hides but keeps its height, so the peek never
 * shifts; the full strip takes over in the sheet's body. The section's name is for screen readers:
 * the field and the numbers say it on screen.
 */
/** The section's name, for screen readers. */
function title({ onMap }: Page): string {
  if (onMap) {
    return "Live events";
  }
  return "Countries";
}

function SheetHeader(): ReactElement {
  const detent = use(RestingDetent);
  const { active } = useMapSearch();
  const cells = useCells();
  const page = usePage();
  return (
    <div className="flex flex-col gap-2 px-4 pb-3">
      {/* Sources shows its own title, with Done, where the field would be. */}
      {page.sources ? <SourcesTitle /> : <h1 className="sr-only">{title(page)}</h1>}
      {page.fieldless ? undefined : (
        <div className="flex items-center gap-1">
          <div className="min-w-0 flex-1">
            <MapSearchBox cancel />
          </div>
          {/* Sources opens from the sheet's header (DESIGN.md §10 shells); the results need the room. */}
          {active ? undefined : <SourcesButton />}
        </div>
      )}
      {page.sources ? undefined : (
        <div className={detent === "peek" && !active ? undefined : "invisible"} data-testid="sheet-strip-line">
          <CompactStatStrip cells={cells} />
        </div>
      )}
    </div>
  );
}

export { SheetHeader };

import type { ReactElement, ReactNode } from "react";

import { MapSearchBox } from "./MapSearchBox.tsx";
import { useMapSearch } from "./mapSearchContext.ts";
import { SearchPanel } from "./SearchPanel.tsx";
import { usePage } from "./usePage.ts";

/**
 * Under the sidebar's nav: the search field, then the open section, or search's results while
 * searching. The section stays mounted under the results, so a country keeps its filter and camera.
 */
function SidebarBody({ children }: { children: ReactNode }): ReactElement {
  const { active } = useMapSearch();
  // The Countries list has its own filter, as on the Mac; one field is enough.
  const { listing } = usePage();
  const searching = active && !listing;
  return (
    <div data-search-area className="mt-3 flex min-h-0 flex-1 flex-col border-t border-hairline">
      {listing ? undefined : (
        <div className="px-3 pt-3">
          <MapSearchBox />
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-y-auto">
        {searching ? <SearchPanel /> : undefined}
        <div hidden={searching}>{children}</div>
      </div>
    </div>
  );
}

export { SidebarBody };

import type { ReactElement, ReactNode } from "react";

import { MapSearchBox } from "./MapSearchBox.tsx";
import { useMapSearch } from "./mapSearchContext.ts";
import { SearchPanel } from "./SearchPanel.tsx";

/** Under the sidebar's nav: the search field, then the open section, or search's results while searching. */
function SidebarBody({ children }: { children: ReactNode }): ReactElement {
  const { active } = useMapSearch();
  return (
    <div data-search-area className="mt-3 flex min-h-0 flex-1 flex-col border-t border-hairline">
      <div className="px-3 pt-3">
        <MapSearchBox />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">{active ? <SearchPanel /> : children}</div>
    </div>
  );
}

export { SidebarBody };

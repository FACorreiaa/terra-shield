import type { ReactElement, ReactNode } from "react";
import { use } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import { SheetStrip } from "./PhoneParts.tsx";
import { SearchPanel } from "./SearchPanel.tsx";
import { SelectedDetail } from "./SelectedDetail.tsx";
import { RestingDetent } from "./sheetDetents.ts";

/** The section, under the full strip from half height up. */
function SectionBody({ children }: { children: ReactNode }): ReactElement {
  const detent = use(RestingDetent);
  return (
    <>
      {detent === "peek" ? undefined : <SheetStrip />}
      {children}
    </>
  );
}

/**
 * What the sheet's body shows: search's results while searching (they win over a selection, as in
 * MapSheet.swift), else the selected event, else the section. The disclaimer closes it, so it is
 * always one scroll away; the event's detail carries its own.
 */
function SheetBody({ children }: { children: ReactNode }): ReactElement {
  const { selectedId } = useLiveMap();
  const { active } = useMapSearch();
  if (!active && selectedId !== undefined) {
    return <SelectedDetail />;
  }
  return (
    <>
      {active ? <SearchPanel /> : <SectionBody>{children}</SectionBody>}
      <footer className="border-t border-hairline px-4 py-4">
        <DisclaimerBanner variant="footer" />
      </footer>
    </>
  );
}

export { SheetBody };

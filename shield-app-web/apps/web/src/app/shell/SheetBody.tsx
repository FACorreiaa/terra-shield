import type { ReactElement, ReactNode } from "react";
import { use } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import { SheetStrip } from "./PhoneParts.tsx";
import { SearchPanel } from "./SearchPanel.tsx";
import { SelectedDetail } from "./SelectedDetail.tsx";
import { RestingDetent } from "./sheetDetents.ts";
import { usePage } from "./usePage.ts";

/** The section, under the map's full strip from half height up; Countries has its own numbers. */
function SectionBody({ children }: { children: ReactNode }): ReactElement {
  const detent = use(RestingDetent);
  const { onMap } = usePage();
  return (
    <>
      {detent === "peek" || !onMap ? undefined : <SheetStrip />}
      {children}
    </>
  );
}

/**
 * What the sheet's body shows: search's results while searching (they win over a selection, as in
 * MapSheet.swift), else the selected event, else the section. The section stays mounted under
 * them, so a country keeps its filter and camera. The disclaimer closes it, so it is always one
 * scroll away; the event's detail carries its own.
 */
function SheetBody({ children }: { children: ReactNode }): ReactElement {
  const { selectedId } = useLiveMap();
  const { active } = useMapSearch();
  const detail = !active && selectedId !== undefined;
  // Sources carries the disclaimer at its top, as in the apps; once is enough.
  const { sources } = usePage();
  return (
    <>
      {active ? <SearchPanel /> : undefined}
      {detail ? <SelectedDetail /> : undefined}
      <div hidden={active || detail}>
        <SectionBody>{children}</SectionBody>
      </div>
      {detail || sources ? undefined : (
        <footer className="border-t border-hairline px-4 py-4">
          <DisclaimerBanner variant="footer" />
        </footer>
      )}
    </>
  );
}

export { SheetBody };

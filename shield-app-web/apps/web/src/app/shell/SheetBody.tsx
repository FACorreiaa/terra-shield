import type { ReactElement, ReactNode } from "react";
import { use, useCallback } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { EventSummary } from "@/features/events/EventSummary.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useMapSearch } from "./mapSearchContext.ts";
import { SheetStrip } from "./PhoneParts.tsx";
import { SearchPanel } from "./SearchPanel.tsx";
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
 * always one scroll away.
 */
function SheetBody({ children }: { children: ReactNode }): ReactElement {
  const { selected, select, now } = useLiveMap();
  const { active } = useMapSearch();
  const close = useCallback(() => {
    select(undefined);
  }, [select]);
  let content: ReactNode = <SectionBody>{children}</SectionBody>;
  if (active) {
    content = <SearchPanel />;
  } else if (selected !== undefined) {
    content = <EventSummary event={selected} now={now} onClose={close} />;
  }
  return (
    <>
      {content}
      <footer className="border-t border-hairline px-4 py-4">
        <DisclaimerBanner variant="footer" />
      </footer>
    </>
  );
}

export { SheetBody };

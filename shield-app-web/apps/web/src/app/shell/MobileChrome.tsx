import type { ReactElement, ReactNode } from "react";
import { useCallback, useLayoutEffect, useState } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { EventSummary } from "@/features/events/EventSummary.tsx";

import { BottomSheet } from "./BottomSheet.tsx";
import type { Insets } from "./insets.ts";
import { PHONE_TOP_INSET } from "./insets.ts";
import { useLiveMap } from "./liveMapContext.ts";
import { PhoneTopStack, SheetHeader, SheetStrip } from "./PhoneParts.tsx";
import type { Detent, Heights } from "./sheetDetents.ts";
import { RestingDetent } from "./sheetDetents.ts";
import { StatusCapsule } from "./StatusCapsule.tsx";

interface MobileChromeProps {
  /** The open section's body (the route), shown in the sheet. */
  children: ReactNode;
  onInsets: (insets: Insets) => void;
}

/**
 * The phone shell's chrome (< 1024 px, DESIGN.md §10 shells after Main.dc.html): the status capsule
 * and the filter chips on top and the sheet with three detents. Selecting an event lifts the sheet
 * to medium to show it. The disclaimer closes the sheet's content, so it is always one scroll away.
 */
function MobileChrome({ children, onInsets }: MobileChromeProps): ReactElement {
  const { selected, select, now } = useLiveMap();
  const [detent, setDetent] = useState<Detent>("peek");
  const [shownId, setShownId] = useState(selected?.id);
  const close = useCallback(() => {
    select(undefined);
  }, [select]);

  // A new selection lifts the sheet from peek so the event shows (adjusted during render, not in an effect).
  if (selected?.id !== shownId) {
    setShownId(selected?.id);
    if (selected !== undefined && detent === "peek") {
      setDetent("medium");
    }
  }

  // The top inset holds from the start; the sheet's arrives once it has measured itself.
  useLayoutEffect(() => {
    onInsets({ top: PHONE_TOP_INSET, right: 0, bottom: 0, left: 0 });
  }, [onInsets]);
  const rest = useCallback(
    (height: number, heights: Heights) => {
      // Past medium the map is mostly covered; centring in the sliver left would fling it about.
      onInsets({ top: PHONE_TOP_INSET, right: 0, bottom: Math.min(height, heights.medium), left: 0 });
    },
    [onInsets],
  );

  return (
    <RestingDetent value={detent}>
      <StatusCapsule />
      <PhoneTopStack />
      <BottomSheet label="Events" Header={SheetHeader} detent={detent} onDetentChange={setDetent} onRest={rest}>
        {selected === undefined ? (
          <>
            {detent === "peek" ? undefined : <SheetStrip />}
            {children}
          </>
        ) : (
          <EventSummary event={selected} now={now} onClose={close} />
        )}
        <footer className="border-t border-hairline px-4 py-4">
          <DisclaimerBanner variant="footer" />
        </footer>
      </BottomSheet>
    </RestingDetent>
  );
}

export { MobileChrome };

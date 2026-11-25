import type { ReactElement, ReactNode } from "react";
import { useCallback, useLayoutEffect, useState } from "react";

import { BottomSheet } from "./BottomSheet.tsx";
import type { Insets } from "./insets.ts";
import { PHONE_TOP_INSET } from "./insets.ts";
import { useLiveMap } from "./liveMapContext.ts";
import { PhoneTopStack } from "./PhoneParts.tsx";
import { SheetBody } from "./SheetBody.tsx";
import type { Detent, Heights } from "./sheetDetents.ts";
import { RestingDetent, SheetDetentSetter } from "./sheetDetents.ts";
import { SheetHeader } from "./SheetHeader.tsx";
import { StatusCapsule } from "./StatusCapsule.tsx";
import { useSearchDetent } from "./useSearchDetent.ts";

interface MobileChromeProps {
  /** The open section's body (the route), shown in the sheet. */
  children: ReactNode;
  onInsets: (insets: Insets) => void;
}

/**
 * The phone shell's chrome (< 1024 px, DESIGN.md §10 shells after Main.dc.html): the status capsule
 * and the filter chips on top and the sheet with three detents. Selecting an event lifts the sheet
 * to medium to show it; searching opens it to large.
 */
function MobileChrome({ children, onInsets }: MobileChromeProps): ReactElement {
  const { selectedId } = useLiveMap();
  // A shared link opens with its event at half height, as a selection made here would.
  const [detent, setDetent] = useState<Detent>(selectedId === undefined ? "peek" : "medium");
  const [shownId, setShownId] = useState(selectedId);

  // A new selection lifts the sheet from peek so the event shows (adjusted during render, not in an effect).
  if (selectedId !== shownId) {
    setShownId(selectedId);
    if (selectedId !== undefined && detent === "peek") {
      setDetent("medium");
    }
  }
  useSearchDetent(detent, setDetent, selectedId);

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
      <SheetDetentSetter value={setDetent}>
        <StatusCapsule />
        <PhoneTopStack />
        <BottomSheet label="Events" Header={SheetHeader} detent={detent} onDetentChange={setDetent} onRest={rest}>
          <SheetBody>{children}</SheetBody>
        </BottomSheet>
      </SheetDetentSetter>
    </RestingDetent>
  );
}

export { MobileChrome };

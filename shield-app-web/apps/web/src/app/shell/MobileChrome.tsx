import type { ReactElement, ReactNode } from "react";
import { useCallback, useLayoutEffect, useState } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { relative } from "@/domain/freshness.ts";
import { EventSummary } from "@/features/events/EventSummary.tsx";

import { BottomSheet } from "./BottomSheet.tsx";
import { Brand } from "./Brand.tsx";
import type { Insets } from "./insets.ts";
import { CAPSULE_INSET } from "./insets.ts";
import { useLiveMap } from "./liveMapContext.ts";
import type { Detent, Heights } from "./sheetDetents.ts";

interface MobileChromeProps {
  /** The open section's body (the route), shown in the sheet. */
  children: ReactNode;
  onInsets: (insets: Insets) => void;
}

/** The top capsule: nav mark and how fresh the map's data is (the live dot arrives with W6). */
function StatusCapsule(): ReactElement {
  const { events, now } = useLiveMap();
  const loadedAt = events.dataUpdatedAt;
  return (
    <header className="absolute inset-x-3 top-3 z-20 flex h-11 items-center justify-between rounded-pill border border-hairline-strong pr-4 pl-2 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none">
      <Brand />
      {loadedAt === 0 ? undefined : (
        <span className="font-mono text-meta text-muted">Updated {relative(new Date(loadedAt), now)}</span>
      )}
    </header>
  );
}

/** What the peek detent shows until the search field and compact strip land (W4). */
function SheetHeader(): ReactElement {
  const { list, clusters, events } = useLiveMap();
  const inClusters = clusters.reduce((sum, cluster) => sum + cluster.count, 0);
  const shown = events.data === undefined ? "–" : `${list.length + inClusters}${events.data.truncated ? "+" : ""}`;
  return (
    <div className="flex items-baseline justify-between px-4 pb-3">
      <h1 className="text-title text-ink">Live events</h1>
      <span className="font-mono text-meta text-muted">{shown} in view</span>
    </div>
  );
}

/**
 * The phone shell's chrome (< 1024 px, DESIGN.md §10 shells after Main.dc.html): the status capsule
 * on top and the sheet with three detents. Selecting an event lifts the sheet to medium to show it.
 * The disclaimer closes the sheet's content, so it is always one scroll away.
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

  // The capsule's inset holds from the start; the sheet's arrives once it has measured itself.
  useLayoutEffect(() => {
    onInsets({ top: CAPSULE_INSET, right: 0, bottom: 0, left: 0 });
  }, [onInsets]);
  const rest = useCallback(
    (height: number, heights: Heights) => {
      // Past medium the map is mostly covered; centring in the sliver left would fling it about.
      onInsets({ top: CAPSULE_INSET, right: 0, bottom: Math.min(height, heights.medium), left: 0 });
    },
    [onInsets],
  );

  return (
    <>
      <StatusCapsule />
      <BottomSheet label="Events" Header={SheetHeader} detent={detent} onDetentChange={setDetent} onRest={rest}>
        {selected === undefined ? children : <EventSummary event={selected} now={now} onClose={close} />}
        <footer className="border-t border-hairline px-4 py-4">
          <DisclaimerBanner variant="footer" />
        </footer>
      </BottomSheet>
    </>
  );
}

export { MobileChrome };

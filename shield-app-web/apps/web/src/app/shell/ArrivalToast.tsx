import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";
import { useCallback, useState } from "react";

import { NewEventToast, ToastAnnouncer } from "@/features/monitor/NewEventToast.tsx";

import { useLiveMap } from "./liveMapContext.ts";

/** Where focus goes when the toast that held it is dismissed: the Filters button just above it. */
function focusFilters(): void {
  document.querySelector<HTMLElement>("[data-filters-button]")?.focus();
}

/**
 * The toast slot at the top of the map: the newest severe arrival, opened from anywhere on the map
 * with `choose` (it may sit in a cluster, out of the map's list). `className` wraps the toast, and
 * only while there is one, so an empty slot takes no room; the announcer is always mounted.
 */
function ArrivalToast({ className }: { className?: string }): ReactElement {
  const { toast, dismissToast, holdToast, choose } = useLiveMap();
  // Focus inside the toast: a newer toast replacing it takes focus over, and a dismissal hands it on.
  const [focused, setFocused] = useState(false);
  const hold = useCallback(
    (held: boolean) => {
      setFocused(held && document.activeElement?.closest("[data-arrival-toast]") !== null);
      holdToast(held);
    },
    [holdToast],
  );
  const open = useCallback(
    (event: HazardEvent) => {
      choose(event);
      dismissToast();
    },
    [choose, dismissToast],
  );
  const dismiss = useCallback(() => {
    if (focused) {
      focusFilters();
    }
    setFocused(false);
    dismissToast();
  }, [focused, dismissToast]);
  return (
    <>
      <ToastAnnouncer event={toast} />
      {toast === undefined ? undefined : (
        <div data-arrival-toast className={className}>
          <NewEventToast
            key={toast.id}
            event={toast}
            onOpen={open}
            onDismiss={dismiss}
            onHold={hold}
            takeFocus={focused}
          />
        </div>
      )}
    </>
  );
}

export { ArrivalToast };

import { Link, useCanGoBack, useNavigate, useRouter } from "@tanstack/react-router";
import { RadioTower } from "lucide-react";
import type { ReactElement } from "react";
import { use, useCallback, useEffect, useRef } from "react";

import { RestingDetent, SheetDetentSetter } from "./sheetDetents.ts";
import { focusLost } from "./useDetailFocus.ts";

const NO_EVENT = {};

// Done gives keyboard focus back to the header's Sources button, which mounts again as Sources
// closes; set by Done, read once by the button.
let returning = false;

/** Opens Sources at full height; an open event closes, or the sheet would keep showing it. */
function SourcesButton(): ReactElement {
  const button = useRef<HTMLAnchorElement>(null);
  useEffect(() => {
    if (returning) {
      returning = false;
      button.current?.focus();
    }
  }, []);
  return (
    <Link
      ref={button}
      to="/sources"
      search={NO_EVENT}
      aria-label="Sources"
      title="Sources (5)"
      className="flex size-11 shrink-0 items-center justify-center rounded-sm text-muted hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
    >
      <RadioTower aria-hidden className="size-5" />
    </Link>
  );
}

/**
 * Sources takes the sheet at full height, as the apps' Sources sheet does, and gives back the
 * height it found when it closes, unless the person has moved the sheet since.
 */
function useSheetAtLarge(): void {
  const setDetent = use(SheetDetentSetter);
  const resting = use(RestingDetent);
  const found = useRef(resting);
  const latest = useRef(resting);
  useEffect(() => {
    latest.current = resting;
  }, [resting]);
  useEffect(() => {
    setDetent?.("large");
    const was = found.current;
    const current = latest;
    return () => {
      if (current.current === "large") {
        setDetent?.(was);
      }
    };
  }, [setDetent]);
}

/**
 * The phone's Sources title in the sheet's pinned header, with Done. Done goes back where Sources
 * was opened from (an event, a country), as the iPhone's Done dismisses its sheet; opened from a
 * link, it goes to the map.
 */
function SourcesTitle(): ReactElement {
  useSheetAtLarge();
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    // The button that opened it has gone with the field: keyboard focus starts on the title.
    if (focusLost()) {
      heading.current?.focus();
    }
  }, []);
  const router = useRouter();
  const canGoBack = useCanGoBack();
  const navigate = useNavigate();
  const done = useCallback(() => {
    returning = true;
    if (canGoBack) {
      router.history.back();
      return;
    }
    void navigate({ to: "/map", search: NO_EVENT });
  }, [canGoBack, router, navigate]);
  return (
    <div className="flex items-center justify-between">
      <h1 ref={heading} tabIndex={-1} className="text-title text-ink outline-none">
        Sources
      </h1>
      <button
        type="button"
        onClick={done}
        className="inline-flex min-h-11 items-center rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
      >
        Done
      </button>
    </div>
  );
}

export { SourcesButton, SourcesTitle };

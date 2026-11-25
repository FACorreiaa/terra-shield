import { createFileRoute, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";
import { z } from "zod";

import { AppFrame } from "@/app/shell/AppFrame.tsx";
import { LiveMapProvider } from "@/app/shell/LiveMapProvider.tsx";
import { MapSearchProvider } from "@/app/shell/MapSearchProvider.tsx";

/**
 * The app's pathless layout. Every map-backed section (`/map`, later `/c/$iso2`, `/sources`, …)
 * renders inside it, so the map and its state stay put while the section changes.
 */
function AppLayout(): ReactElement {
  return (
    <LiveMapProvider>
      <MapSearchProvider>
        <AppFrame>
          <Outlet />
        </AppFrame>
      </MapSearchProvider>
    </LiveMapProvider>
  );
}

/** Longer than any id the server issues; a longer one is not an id. */
const MAX_ID_LENGTH = 64;
const EventId = z.string().min(1).max(MAX_ID_LENGTH);

interface AppSearch {
  event?: string | undefined;
}

/** `?event=<id>`: the selected event, on any section. Anything malformed is dropped, never an error. */
function appSearch(search: Readonly<Record<string, unknown>>): AppSearch {
  const parsed = EventId.safeParse(search["event"]);
  return parsed.success ? { event: parsed.data } : {};
}

export const Route = createFileRoute("/_app")({ component: AppLayout, validateSearch: appSearch });

import { createFileRoute, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";

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

export const Route = createFileRoute("/_app")({ component: AppLayout });

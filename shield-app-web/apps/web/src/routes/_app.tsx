import { createFileRoute, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { AppShell } from "@/components/layout/AppShell.tsx";

/** The app's pathless layout: every map-backed screen (`/map`, later `/e/$id`, `/c/$iso2`) shares it. */
function AppLayout(): ReactElement {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

export const Route = createFileRoute("/_app")({ component: AppLayout });

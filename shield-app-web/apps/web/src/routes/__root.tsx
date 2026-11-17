import { createRootRoute, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { AppShell } from "@/components/layout/AppShell.tsx";

function RootLayout(): ReactElement {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

export const Route = createRootRoute({ component: RootLayout });

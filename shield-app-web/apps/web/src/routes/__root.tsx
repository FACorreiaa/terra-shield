import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { SiteFooter } from "@/components/SiteFooter.tsx";
import { SiteHeader } from "@/components/SiteHeader.tsx";

function RootLayout(): ReactElement {
  return <Outlet />;
}

/** An unknown path says so plainly and offers the map; it never pretends a place is safe. */
function NotFound(): ReactElement {
  return (
    <div className="flex min-h-full flex-col bg-canvas">
      <SiteHeader />
      <main className="grid flex-1 place-items-center px-6 py-16 text-center">
        <div className="flex max-w-sm flex-col items-center gap-3">
          <p className="text-micro tracking-[0.08em] text-faint uppercase">Not found</p>
          <h1 className="text-title text-ink">There is no page at this address.</h1>
          <Link
            to="/map"
            className="mt-2 inline-flex min-h-11 items-center rounded-sm bg-accent px-4 text-body font-semibold text-on-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            Open the live map
          </Link>
        </div>
      </main>
      <SiteFooter />
    </div>
  );
}

export const Route = createRootRoute({ component: RootLayout, notFoundComponent: NotFound });

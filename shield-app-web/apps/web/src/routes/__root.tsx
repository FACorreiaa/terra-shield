import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import type { ReactElement } from "react";

function RootLayout(): ReactElement {
  return <Outlet />;
}

/** An unknown path says so plainly and offers the map; it never pretends a place is safe. */
function NotFound(): ReactElement {
  return (
    <main className="grid h-full place-items-center bg-canvas px-6 text-center">
      <div className="flex max-w-sm flex-col items-center gap-3">
        <p className="text-micro tracking-[0.08em] text-faint uppercase">Not found</p>
        <h1 className="text-title text-ink">There is no page at this address.</h1>
        <Link
          to="/map"
          className="mt-2 rounded-sm bg-accent px-4 py-2.5 text-body font-semibold text-on-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        >
          Open the live map
        </Link>
      </div>
    </main>
  );
}

export const Route = createRootRoute({ component: RootLayout, notFoundComponent: NotFound });

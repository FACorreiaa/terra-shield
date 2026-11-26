import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";

/**
 * The web landing hero (DESIGN.md §11, §12): the full lockup on Orbital Earth under a vignette.
 * W8 completes it with the live stat strip, the four kinds and the sources.
 */
function LandingPage(): ReactElement {
  return (
    <div className="relative flex min-h-full flex-col bg-canvas">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[url(/brand/orbital-earth.svg)] bg-cover bg-center opacity-60"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,transparent_0%,var(--color-canvas)_72%)]"
      />
      <main className="relative flex flex-1 flex-col items-center justify-center gap-6 px-6 py-16 text-center">
        <img src="/brand/lockup.webp" alt="Aegis Terra" width={320} height={320} className="size-64 sm:size-80" />
        <p className="text-micro tracking-[0.08em] text-muted uppercase">Global hazard intelligence</p>
        <h1 className="max-w-xl text-title text-ink">Global hazard intelligence. Official sources, live.</h1>
        <Link
          to="/map"
          className="mt-2 inline-flex min-h-11 items-center rounded-sm bg-accent px-5 text-body font-semibold text-on-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        >
          Open the live map
        </Link>
      </main>
      <footer className="relative border-t border-hairline px-6 py-4 text-center">
        <DisclaimerBanner variant="footer" />
      </footer>
    </div>
  );
}

export { LandingPage };

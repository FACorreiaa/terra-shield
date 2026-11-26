import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { SiteFooter } from "@/components/SiteFooter.tsx";
import { SiteHeader } from "@/components/SiteHeader.tsx";

import { KindsSection } from "./KindsSection.tsx";
import { SourcesSection } from "./SourcesSection.tsx";
import { WorldStrip } from "./WorldStrip.tsx";

const CTA =
  "inline-flex min-h-11 items-center rounded-sm bg-accent px-5 text-body font-semibold text-on-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

/**
 * The full lockup on Orbital Earth at the apps' 45 % under a vignette (DESIGN.md §11, §12), the way
 * in, and the world's numbers. The vignette never clears to the art: the words over it sit on a scrim.
 */
function Hero(): ReactElement {
  return (
    <div className="relative">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[url(/brand/orbital-earth.svg)] bg-cover bg-center opacity-45"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,color-mix(in_oklab,var(--color-canvas)_55%,transparent)_0%,var(--color-canvas)_72%)]"
      />
      <div className="relative mx-auto flex min-h-[calc(100svh-3rem)] max-w-3xl flex-col items-center justify-center gap-6 px-4 py-12 text-center">
        <img
          src="/brand/lockup-576.webp"
          alt="Aegis Terra"
          width={576}
          height={576}
          fetchPriority="high"
          className="size-56 sm:size-72"
        />
        <p className="text-micro tracking-[0.08em] text-muted uppercase">Global hazard intelligence</p>
        <h1 className="max-w-xl text-title text-ink">Global hazard intelligence. Official sources, live.</h1>
        <Link to="/map" className={CTA}>
          Open the live map
        </Link>
        <WorldStrip />
      </div>
    </div>
  );
}

/**
 * The web's front door at `/` (owner, 2026-11-19), in place of the apps' three onboarding cards:
 * the hero (card 1), the kinds (card 2), the sources, and the disclaimer in the footer (card 3).
 * No map here, and no live stream: the map loads when it is opened.
 */
function LandingPage(): ReactElement {
  return (
    <div className="flex min-h-full flex-col bg-canvas">
      <SiteHeader />
      <main className="flex flex-1 flex-col">
        <Hero />
        <KindsSection />
        <SourcesSection />
        <section className="flex flex-col items-center gap-4 px-4 pt-4 pb-16 text-center">
          <Link to="/map" className={CTA}>
            Show me the map
          </Link>
        </section>
      </main>
      <SiteFooter />
    </div>
  );
}

export { LandingPage };

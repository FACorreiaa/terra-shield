import type { EventKind } from "@shield/api-client";
import type { ReactElement } from "react";

import { KindChip } from "@/features/events/KindChip.tsx";

/** Every kind, satellite heat detections included (the map's heat layer), in the apps' order. */
const KINDS: readonly EventKind[] = ["observed", "warning", "forecast", "detection"];

/** OnboardingView.swift's words, as written. */
const KINDS_LINE = "Know what you're looking at: observed, official warning, forecast, satellite detection.";

/** Onboarding's second card on the web (OnboardingView.swift `kinds`): the Sentinel beside the kinds. */
function KindsSection(): ReactElement {
  return (
    <section aria-labelledby="kinds-heading" className="mx-auto flex w-full max-w-3xl flex-col gap-8 px-4 py-16">
      <h2 id="kinds-heading" className="text-title text-ink">
        {KINDS_LINE}
      </h2>
      <div className="flex items-center gap-6">
        {/* The Sentinel at most once a screen, never under 120 px tall (DESIGN.md §11). */}
        <img
          src="/brand/sentinel.webp"
          alt=""
          width={768}
          height={768}
          loading="lazy"
          decoding="async"
          className="size-36 shrink-0 sm:size-44"
        />
        <ul className="flex flex-col items-start gap-2" aria-label="Kinds of record">
          {KINDS.map((kind) => (
            <li key={kind}>
              <KindChip kind={kind} />
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

export { KindsSection };

import { Plane } from "lucide-react";
import type { ReactElement } from "react";

import { flightsAttribution, flightsHeadline, flightsLines } from "@/domain/aircraft.ts";
import { useTick } from "@/lib/useTick.ts";
import { cn } from "@/lib/utils.ts";

import { useFlightsLayer } from "./flightsContext.ts";

/** What the key says aloud: why there are no positions, never the count or age as they tick. */
function announcement({ list, failed, needsZoom }: Parameters<typeof flightsHeadline>[0]): string {
  if (needsZoom) {
    return "Flights · zoom in to see them";
  }
  if (!failed) {
    return "";
  }
  return list === undefined ? "Flights didn’t load" : "The last refresh of flights failed.";
}

/**
 * The flights layer's key, under the map's notices while it is on (`FlightsLegend`, Flights.swift):
 * how many, how old, whether complete, and always the ODbL attribution (ADR 28).
 */
function FlightsLegend(): ReactElement {
  const { list, failed, needsZoom } = useFlightsLayer();
  const now = useTick();
  const state = { list, failed, needsZoom };
  const headline = flightsHeadline(state);
  return (
    <div
      data-flights-legend
      className="flex w-fit max-w-[min(100%,22.5rem)] flex-col gap-1 rounded-md border border-hairline-strong px-3 py-2 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <span className={cn("flex items-start gap-2 text-meta font-semibold", failed ? "text-warning" : "text-ink")}>
        <Plane className="mt-0.5 size-3 shrink-0" strokeWidth={2.5} aria-hidden />
        {headline}
      </span>
      {flightsLines(state, now).map((line) => (
        <span key={line} className="text-meta text-muted">
          {line}
        </span>
      ))}
      <span className="text-micro text-muted">{flightsAttribution(list)}</span>
      {/* Zoom in and failures are announced once; the count and age stay quiet as they change. */}
      <output className="sr-only">{announcement(state)}</output>
    </div>
  );
}

export { FlightsLegend };

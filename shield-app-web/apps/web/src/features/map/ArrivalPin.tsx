import type { HazardEvent } from "@shield/api-client";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReactElement } from "react";
import { useCallback, useEffect, useEffectEvent, useState } from "react";

import { motionTokens } from "@/design/tokens.gen.ts";

import { ARRIVING, EVENTS_SOURCE, isMarker, markerState } from "./eventLayer.ts";
import { MapMarker } from "./MapMarker.tsx";
import type { MarkerKind } from "./markerArt.ts";
import { MarkerImage } from "./MarkerImage.tsx";

/** The copy stays this long after the animation, over the symbol shown under it, so the hand-over shows no gap. */
const OVERLAP_MS = 300;
/**
 * The animation's end hands over; this only backs it up, for an animation that never runs (a hidden
 * tab, the globe's far side). Generous: a busy page starts the animation late, and the symbol must
 * not show up mid-way.
 */
const FALLBACK_MS = 2000;
/** At most this many arrivals animate at once; any more simply appear. */
const MAX_PINS = 20;

interface ArrivalCallbacks {
  /** The arrival began animating. */
  onStarted: (id: string) => void;
  /** The arrival has animated in. */
  onArrived: (id: string) => void;
}

interface ArrivalPinProps {
  map: MapLibreMap;
  arrival: ArrivalCallbacks;
  event: HazardEvent & { kind: MarkerKind };
  staleSources: ReadonlySet<string>;
}

/** Shows or hides the event's symbol under its animating copy; quiet when the style is mid-reload. */
function waitSymbol(map: MapLibreMap, id: string, waiting: boolean): void {
  if (map.getSource(EVENTS_SOURCE) !== undefined) {
    map.setFeatureState({ source: EVENTS_SOURCE, id }, { [ARRIVING]: waiting });
  }
}

/**
 * A live arrival fading in and growing from 0.6 (DESIGN.md §13 `arrive`), as a DOM copy of its
 * marker over its symbol, which waits unseen; under reduced motion it only fades. The animation's
 * end shows the symbol (a timer could beat an animation a busy page started late), then the copy goes.
 */
function ArrivalPin({ map, event, staleSources, arrival }: ArrivalPinProps): ReactElement {
  const { id } = event;
  // The clock runs once per pin (keyed by event): new callbacks from another arrival don't restart it.
  const start = useEffectEvent(() => {
    arrival.onStarted(id);
    waitSymbol(map, id, true);
  });
  const reveal = useEffectEvent(() => {
    waitSymbol(map, id, false);
  });
  const leave = useEffectEvent(() => {
    arrival.onArrived(id);
  });
  const [ended, setEnded] = useState(false);
  const end = useCallback(() => {
    setEnded(true);
  }, []);
  useEffect(() => {
    start();
    const fallback = setTimeout(end, motionTokens.arrive + FALLBACK_MS);
    return () => {
      clearTimeout(fallback);
      reveal();
    };
  }, [end]);
  useEffect(() => {
    if (!ended) {
      return undefined;
    }
    reveal();
    const leaving = setTimeout(leave, OVERLAP_MS);
    return () => {
      clearTimeout(leaving);
    };
  }, [ended]);
  return (
    <MapMarker map={map} longitude={event.centroid.longitude} latitude={event.centroid.latitude}>
      <MarkerImage
        event={event}
        state={markerState(event, staleSources)}
        className="arrive pointer-events-none"
        onAnimationEnd={end}
      />
    </MapMarker>
  );
}

interface ArrivalPinsProps {
  map: MapLibreMap;
  arrival: ArrivalCallbacks;
  events: readonly HazardEvent[];
  staleSources: ReadonlySet<string>;
  arriving: ReadonlySet<string>;
}

/** The arrivals among the drawn markers; one not drawn yet (a reload on its way) waits for its marker. */
function ArrivalPins({ map, events, staleSources, arriving, arrival }: ArrivalPinsProps): ReactElement {
  const animating =
    arriving.size === 0 ? [] : events.filter((event) => arriving.has(event.id) && isMarker(event)).slice(0, MAX_PINS);
  return (
    <>
      {animating.map((event) =>
        isMarker(event) ? (
          <ArrivalPin key={event.id} map={map} event={event} staleSources={staleSources} arrival={arrival} />
        ) : undefined,
      )}
    </>
  );
}

export type { ArrivalCallbacks };
export { ArrivalPins };

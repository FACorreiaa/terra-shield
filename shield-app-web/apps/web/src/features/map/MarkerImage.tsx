import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";
import { useMemo } from "react";

import { eventType } from "@/domain/taxonomy.ts";

import type { MarkerKind, MarkerState } from "./markerArt.ts";
import { markerFrame } from "./markerArt.ts";
import { markerUrl } from "./markerImages.ts";

interface MarkerImageProps {
  event: HazardEvent & { kind: MarkerKind };
  /** As the symbol layer draws it: faded for a stale source or a resolved event. */
  state?: MarkerState;
  className?: string;
  onAnimationEnd?: () => void;
}

/** An event's map marker as a DOM image, the same art the map's symbol layer draws. */
function MarkerImage({ event, state = "live", className, onAnimationEnd }: MarkerImageProps): ReactElement {
  const source = useMemo(
    () => markerUrl({ type: eventType(event.type), severity: event.severity, kind: event.kind, state }),
    [event.type, event.severity, event.kind, state],
  );
  const frame = markerFrame(event.severity);
  return (
    <img
      alt=""
      src={source}
      width={frame}
      height={frame}
      draggable={false}
      className={className}
      onAnimationEnd={onAnimationEnd}
    />
  );
}

export { MarkerImage };

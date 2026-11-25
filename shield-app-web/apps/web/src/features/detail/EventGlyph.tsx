import type { HazardEvent } from "@shield/api-client";
import { hazardType } from "@shield/api-client";
import type { ReactElement } from "react";
import { useMemo } from "react";

import { alphaTokens, hazardTypeTokens } from "@/design/tokens.gen.ts";
import { glyphMarkup } from "@/features/map/glyphMarkup.ts";
import type { MarkerKind, MarkerState } from "@/features/map/markerArt.ts";
import { markerSvg } from "@/features/map/markerArt.ts";

/** Drawn at twice its size so it stays sharp on high-density screens. */
const PIXEL_RATIO = 2;

interface EventGlyphProps {
  event: Pick<HazardEvent, "type" | "severity" | "kind">;
  state: MarkerState;
  /** Gone: faded and grey without a badge, neither resolved nor waiting on its source. */
  gone?: boolean;
  className: string;
}

/** The event's map marker as a picture: severity's shape and colour, kind's form, the type's glyph. */
function EventGlyph({ event, state, gone = false, className }: EventGlyphProps): ReactElement {
  const source = useMemo(() => {
    // Heat detections draw as dots on the map; here they take the observed form.
    const kind: MarkerKind = event.kind === "detection" ? "observed" : event.kind;
    const glyph = glyphMarkup(hazardTypeTokens[hazardType(event.type)].icon);
    const svg = markerSvg({ severity: event.severity, kind, state, glyph }, alphaTokens.stale, PIXEL_RATIO);
    return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  }, [event.type, event.severity, event.kind, state]);
  return <img src={source} alt="" className={`${className} object-contain ${gone ? "opacity-45 grayscale" : ""}`} />;
}

export { EventGlyph };

import type { Map as MapLibreMap } from "maplibre-gl";

import { severityTokens } from "@/design/tokens.gen.ts";

type Shape = (typeof severityTokens)[keyof typeof severityTokens]["shape"];
type Point = readonly [number, number];

const SIZE = 64;
const PAD = 6;
const MID = SIZE / 2;
const MIN = PAD;
const MAX = SIZE - PAD;
/** The circle and triangle sit slightly inside the box so all four read at the same weight. */
const CIRCLE_INSET = 3;
const TRIANGLE_BASE_INSET = 4;
/** Regular octagon: each corner cut is w / (2 + √2). */
const OCTAGON_CUT = (MAX - MIN) / (2 + Math.SQRT2);
const OUTLINE_WIDTH = 6;
const PIXEL_RATIO = 2;

const polygons: Record<Exclude<Shape, "circle">, readonly Point[]> = {
  diamond: [
    [MID, MIN],
    [MAX, MID],
    [MID, MAX],
    [MIN, MID],
  ],
  triangle: [
    [MID, MIN],
    [MAX, MAX - TRIANGLE_BASE_INSET],
    [MIN, MAX - TRIANGLE_BASE_INSET],
  ],
  octagon: [
    [MIN + OCTAGON_CUT, MIN],
    [MAX - OCTAGON_CUT, MIN],
    [MAX, MIN + OCTAGON_CUT],
    [MAX, MAX - OCTAGON_CUT],
    [MAX - OCTAGON_CUT, MAX],
    [MIN + OCTAGON_CUT, MAX],
    [MIN, MAX - OCTAGON_CUT],
    [MIN, MIN + OCTAGON_CUT],
  ],
};

function trace(ctx: CanvasRenderingContext2D, shape: Shape): void {
  ctx.beginPath();
  if (shape === "circle") {
    ctx.arc(MID, MID, MID - PAD - CIRCLE_INSET, 0, Math.PI * 2);
  } else {
    for (const [px, py] of polygons[shape]) {
      ctx.lineTo(px, py);
    }
  }
  ctx.closePath();
}

function drawShape(shape: Shape, outline: boolean): ImageData | undefined {
  const canvas = document.createElement("canvas");
  canvas.width = SIZE;
  canvas.height = SIZE;
  const ctx = canvas.getContext("2d");
  if (ctx === null) {
    return undefined;
  }
  trace(ctx, shape);
  ctx.fillStyle = "#fff";
  ctx.strokeStyle = "#fff";
  ctx.lineWidth = OUTLINE_WIDTH;
  if (outline) {
    ctx.stroke();
  } else {
    ctx.fill();
  }
  return ctx.getImageData(0, 0, SIZE, SIZE);
}

const variants = Object.values(severityTokens).flatMap(({ shape }) => [
  { shape, outline: false },
  { shape, outline: true },
]);

/**
 * Registers `shape-<name>` (filled) and `shape-<name>-outline` (forecast) as SDF images, so one
 * symbol layer can tint them by severity. Severity is never colour alone (DESIGN.md §4).
 */
function addMarkerImages(map: MapLibreMap): void {
  for (const { shape, outline } of variants) {
    const name = outline ? `shape-${shape}-outline` : `shape-${shape}`;
    const image = map.hasImage(name) ? undefined : drawShape(shape, outline);
    if (image !== undefined) {
      map.addImage(name, image, { sdf: true, pixelRatio: PIXEL_RATIO });
    }
  }
}

export { addMarkerImages };

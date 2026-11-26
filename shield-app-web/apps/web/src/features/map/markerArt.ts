import { colorTokens, elevationTokens, severityColors, severityTokens } from "@/design/tokens.gen.ts";
import type { SeverityKey } from "@/design/tokens.gen.ts";

/**
 * Hazard marker art as SVG (DESIGN.md §4, §5), the web twin of `HazardMarker.swift`:
 * severity sets shape, colour and size; kind sets the form (filled, ringed, outline); the type
 * glyph sits inside. Stale and resolved markers fade and desaturate, with a clock or check badge.
 */

type MarkerKind = "observed" | "warning" | "forecast";
type MarkerState = "live" | "stale" | "resolved";
type Shape = (typeof severityTokens)[SeverityKey]["shape"];

interface MarkerSpec {
  severity: SeverityKey;
  kind: MarkerKind;
  state: MarkerState;
  /** Inner SVG markup of a 24-unit Lucide glyph (paths only, no outer <svg>). */
  glyph: string;
  /**
   * A conflict geocoded only roughly (district, province, area): a dashed ring says the point is
   * not exact (HazardMarker.swift `isApproximate`; docs/attrs.md `locationPrecision`).
   */
  approximate?: boolean;
}

/** Point size of a moderate marker; the others scale from it (DESIGN.md §4). */
const BASE_SIZE = 28;
/** Room around the shape for the shadow, the warning ring and the state badge. */
const MARGIN = 6;
const LUCIDE_VIEWBOX = 24;

const GEOMETRY = {
  edgeInset: 0.5,
  circleInset: 0.06,
  triangleBaseInset: 0.06,
  triangleGlyphOffset: 0.12,
  glyphScale: { circle: 0.52, octagon: 0.52, diamond: 0.42, triangle: 0.38 },
  ringInset: 3,
  ringWidth: 2,
  forecastStroke: 2,
  forecastFillOpacity: 0.85,
  glyphStroke: 2.5,
  badgeRadius: 6.5,
  badgeGlyph: 9,
  /** The approximate ring: 1.6 times the shape, 1.5 px dashed 3–3 at 0.8, plus room for its stroke. */
  approximate: { scale: 1.6, room: 4, width: 1.5, dash: 3, opacity: 0.8 },
} as const;

/** The shape's side in px. */
function markerSize(severity: SeverityKey): number {
  return Math.round(BASE_SIZE * severityTokens[severity].markerScale);
}

/** The marker's frame in px: the shape plus its margin, or the approximate ring when it is wider. */
function markerFrame(severity: SeverityKey, approximate = false): number {
  const size = markerSize(severity);
  const plain = size + MARGIN * 2;
  if (!approximate) {
    return plain;
  }
  const { scale, room } = GEOMETRY.approximate;
  return Math.max(plain, Math.ceil(size * scale + room));
}

/** The dashed ring round a roughly placed marker, in its severity colour, behind the shape. */
function approximateMarkup(severity: SeverityKey, frame: number, size: number): string {
  const { scale, width, dash, opacity } = GEOMETRY.approximate;
  const center = frame / 2;
  const radius = (size * scale) / 2 - width / 2;
  return `<circle cx="${center}" cy="${center}" r="${radius.toFixed(2)}" fill="none" stroke="${severityColors[severity]}" stroke-opacity="${opacity}" stroke-width="${width}" stroke-dasharray="${dash} ${dash}"/>`;
}

function points(list: readonly (readonly [number, number])[]): string {
  return list.map(([px, py]) => `${px.toFixed(2)},${py.toFixed(2)}`).join(" ");
}

/** The shape's outline inside a square box at (origin, origin) of side `size`. */
function shapePath(shape: Shape, origin: number, size: number): string {
  const min = origin + GEOMETRY.edgeInset;
  const max = origin + size - GEOMETRY.edgeInset;
  const mid = origin + size / 2;
  const width = max - min;
  if (shape === "circle") {
    const radius = width / 2 - width * GEOMETRY.circleInset;
    return `<circle cx="${mid}" cy="${mid}" r="${radius.toFixed(2)}"/>`;
  }
  if (shape === "diamond") {
    return `<polygon points="${points([
      [mid, min],
      [max, mid],
      [mid, max],
      [min, mid],
    ])}"/>`;
  }
  if (shape === "triangle") {
    const base = max - width * GEOMETRY.triangleBaseInset;
    return `<polygon points="${points([
      [mid, min],
      [max, base],
      [min, base],
    ])}"/>`;
  }
  const cut = width / (2 + Math.SQRT2);
  return `<polygon points="${points([
    [min + cut, min],
    [max - cut, min],
    [max, min + cut],
    [max, max - cut],
    [max - cut, max],
    [min + cut, max],
    [min, max - cut],
    [min, min + cut],
  ])}"/>`;
}

/** Where the shape is drawn inside the frame. */
interface Box {
  origin: number;
  size: number;
}

function glyphMarkup(spec: MarkerSpec, shape: Shape, { origin, size }: Box): string {
  const glyphSize = size * GEOMETRY.glyphScale[shape];
  const offsetY = shape === "triangle" ? size * GEOMETRY.triangleGlyphOffset : 0;
  const left = origin + (size - glyphSize) / 2;
  const top = origin + (size - glyphSize) / 2 + offsetY;
  const color = spec.kind === "forecast" ? severityColors[spec.severity] : colorTokens.canvas;
  return `<svg x="${left.toFixed(2)}" y="${top.toFixed(2)}" width="${glyphSize.toFixed(2)}" height="${glyphSize.toFixed(2)}" viewBox="0 0 ${LUCIDE_VIEWBOX} ${LUCIDE_VIEWBOX}" fill="none" stroke="${color}" stroke-width="${GEOMETRY.glyphStroke}" stroke-linecap="round" stroke-linejoin="round">${spec.glyph}</svg>`;
}

function bodyMarkup(spec: MarkerSpec, shape: Shape, { origin, size }: Box): string {
  const color = severityColors[spec.severity];
  const outline = shapePath(shape, origin, size);
  if (spec.kind === "forecast") {
    return `<g fill="${colorTokens.canvas}" fill-opacity="${GEOMETRY.forecastFillOpacity}" stroke="${color}" stroke-width="${GEOMETRY.forecastStroke}">${outline}</g>`;
  }
  const filled = `<g fill="${color}">${outline}</g>`;
  if (spec.kind === "warning") {
    const inset = GEOMETRY.ringInset;
    const ring = shapePath(shape, origin + inset, size - inset * 2);
    return `${filled}<g fill="none" stroke="${colorTokens.canvas}" stroke-width="${GEOMETRY.ringWidth}">${ring}</g>`;
  }
  return filled;
}

/** Clock for stale, check for resolved; `muted`, never `ok`: green stays off the map (DESIGN.md §3). */
function badgeMarkup(state: MarkerState, frame: number): string {
  if (state === "live") {
    return "";
  }
  const radius = GEOMETRY.badgeRadius;
  const cx = frame - radius - 1;
  const cy = radius + 1;
  const glyph =
    state === "resolved" ? '<path d="M20 6 9 17l-5-5"/>' : '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>';
  const size = GEOMETRY.badgeGlyph;
  return `<circle cx="${cx}" cy="${cy}" r="${radius}" fill="${colorTokens.muted}"/><svg x="${cx - size / 2}" y="${cy - size / 2}" width="${size}" height="${size}" viewBox="0 0 ${LUCIDE_VIEWBOX} ${LUCIDE_VIEWBOX}" fill="none" stroke="${colorTokens.canvas}" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">${glyph}</svg>`;
}

/** The complete marker as a standalone SVG document, `frame` px square. */
function markerSvg(spec: MarkerSpec, staleOpacity: number, pixelRatio = 1): string {
  const approximate = spec.approximate === true;
  const frame = markerFrame(spec.severity, approximate);
  const size = markerSize(spec.severity);
  const { shape } = severityTokens[spec.severity];
  const faded = spec.state !== "live";
  const shadow = elevationTokens.marker;
  const filters = `<filter id="s" x="-30%" y="-30%" width="160%" height="160%"><feDropShadow dx="0" dy="${shadow.y}" stdDeviation="${shadow.blur / 2}" flood-color="${shadow.color}" flood-opacity="${shadow.opacity}"/></filter><filter id="g"><feColorMatrix type="saturate" values="0"/></filter>`;
  const box = { origin: (frame - size) / 2, size };
  const ring = approximate ? approximateMarkup(spec.severity, frame, size) : "";
  const marker = `${ring}${bodyMarkup(spec, shape, box)}${glyphMarkup(spec, shape, box)}`;
  const group = faded
    ? `<g opacity="${staleOpacity}" filter="url(#g)"><g filter="url(#s)">${marker}</g></g>`
    : `<g filter="url(#s)">${marker}</g>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${frame * pixelRatio}" height="${frame * pixelRatio}" viewBox="0 0 ${frame} ${frame}"><defs>${filters}</defs>${group}${badgeMarkup(spec.state, frame)}</svg>`;
}

/** The style image name a symbol layer asks for: one image per type, severity, kind and state. */
function markerImageName(type: string, spec: Omit<MarkerSpec, "glyph">): string {
  return `marker/${type}/${spec.severity}/${spec.kind}/${spec.state}${spec.approximate === true ? "/approximate" : ""}`;
}

export type { MarkerKind, MarkerSpec, MarkerState };
export { markerFrame, markerImageName, markerSize, markerSvg };

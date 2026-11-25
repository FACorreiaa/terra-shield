import { describe, expect, it } from "vite-plus/test";

import { colorTokens, severityColors } from "@/design/tokens.gen.ts";

import type { MarkerSpec } from "./markerArt.ts";
import { markerFrame, markerImageName, markerSvg } from "./markerArt.ts";

const GLYPH = '<path d="M1 1"/>';
const STALE = 0.45;
const base: MarkerSpec = { severity: "moderate", kind: "observed", state: "live", glyph: GLYPH };
const POINTS = /points="(?<list>[^"]+)"/u;

/** How many corners the marker's polygon has. */
function vertices(spec: MarkerSpec): number {
  return POINTS.exec(markerSvg(spec, STALE))?.groups?.["list"]?.split(" ").length ?? 0;
}

describe(markerSvg, () => {
  it("draws severity as its own shape: circle, diamond, triangle, octagon", () => {
    expect(markerSvg({ ...base, severity: "minor" }, STALE)).toContain("<circle");
    expect(vertices({ ...base, severity: "moderate" })).toBe(4);
    expect(vertices({ ...base, severity: "severe" })).toBe(3);
    expect(vertices({ ...base, severity: "extreme" })).toBe(8);
  });

  it("fills an observed marker in its severity colour with a canvas glyph", () => {
    const svg = markerSvg({ ...base, severity: "severe" }, STALE);
    expect(svg).toContain(`fill="${severityColors.severe}"`);
    expect(svg).toContain(`stroke="${colorTokens.canvas}" stroke-width="2.5"`);
    expect(svg).toContain(GLYPH);
  });

  it("rings an official warning in canvas and outlines a forecast", () => {
    expect(markerSvg({ ...base, kind: "warning" }, STALE)).toContain(
      `fill="none" stroke="${colorTokens.canvas}" stroke-width="2"`,
    );
    const forecast = markerSvg({ ...base, kind: "forecast" }, STALE);
    expect(forecast).toContain(`stroke="${severityColors.moderate}" stroke-width="2"`);
    expect(forecast).toContain(`stroke="${severityColors.moderate}" stroke-width="2.5"`);
  });

  it("fades and desaturates stale and resolved markers, with a muted badge", () => {
    expect(markerSvg(base, STALE)).not.toContain('opacity="0.45"');
    const stale = markerSvg({ ...base, state: "stale" }, STALE);
    expect(stale).toContain('opacity="0.45" filter="url(#g)"');
    expect(stale).toContain(`fill="${colorTokens.muted}"`);
    expect(markerSvg({ ...base, state: "resolved" }, STALE)).toContain("M20 6 9 17l-5-5");
  });

  it("draws at the requested pixel ratio over the same view box", () => {
    const frame = markerFrame("moderate");
    expect(markerSvg(base, STALE, 2)).toContain(
      `width="${frame * 2}" height="${frame * 2}" viewBox="0 0 ${frame} ${frame}"`,
    );
  });
});

describe(markerFrame, () => {
  it("scales with severity, extreme largest", () => {
    expect(markerFrame("minor")).toBeLessThan(markerFrame("moderate"));
    expect(markerFrame("extreme")).toBeGreaterThan(markerFrame("severe"));
  });
});

describe(markerImageName, () => {
  it("names one image per type, severity, kind and state", () => {
    expect(markerImageName("flood", { severity: "severe", kind: "warning", state: "stale" })).toBe(
      "marker/flood/severe/warning/stale",
    );
  });
});

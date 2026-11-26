import { colorTokens } from "@/design/tokens.gen.ts";
import { mix } from "@/lib/color.ts";

/** How far each derived colour moves from its base token toward another (0–1). */
const BLEND = {
  landTowardPanel: 0.72,
  iceTowardFaint: 0.08,
  roadTowardRaised: 0.35,
  majorRoadTowardRaised: 0.6,
  admin1TowardHairline: 0.55,
  minorLabelTowardLand: 0.35,
  waterLabelTowardOcean: 0.45,
  atmosphereTowardAccent: 0.14,
} as const;

interface BasemapPalette {
  ocean: string;
  land: string;
  ice: string;
  road: string;
  roadMajor: string;
  admin0: string;
  admin1: string;
  label: string;
  labelMinor: string;
  waterLabel: string;
  halo: string;
}

function palette(): BasemapPalette {
  const { canvas, panel, raised, hairline, faint } = colorTokens;
  const land = mix(canvas, panel, BLEND.landTowardPanel);
  return {
    ocean: canvas,
    land,
    ice: mix(land, faint, BLEND.iceTowardFaint),
    road: mix(land, raised, BLEND.roadTowardRaised),
    roadMajor: mix(land, raised, BLEND.majorRoadTowardRaised),
    admin0: hairline,
    admin1: mix(land, hairline, BLEND.admin1TowardHairline),
    label: faint,
    labelMinor: mix(faint, land, BLEND.minorLabelTowardLand),
    waterLabel: mix(faint, canvas, BLEND.waterLabelTowardOcean),
    halo: canvas,
  };
}

export type { BasemapPalette };
export { BLEND, palette };

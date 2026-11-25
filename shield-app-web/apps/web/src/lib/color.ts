/** Colour arithmetic on `#RRGGBB` tokens, for canvas-drawn surfaces that cannot use CSS. */

const HEX_RADIX = 16;
const HEX_BYTE = 2;
const CHANNEL_MAX = 255;
const CHANNELS = [0, 1, 2] as const;

function channel(hex: string, index: number): number {
  const start = 1 + index * HEX_BYTE;
  return Number.parseInt(hex.slice(start, start + HEX_BYTE), HEX_RADIX);
}

/** Mixes two tokens: `amount` 0 is `from`, 1 is `to`. Keeps every derived colour traceable to a token. */
function mix(from: string, to: string, amount: number): string {
  const parts = CHANNELS.map((index) => {
    const value = Math.round(channel(from, index) + (channel(to, index) - channel(from, index)) * amount);
    return Math.min(CHANNEL_MAX, Math.max(0, value)).toString(HEX_RADIX).padStart(HEX_BYTE, "0");
  });
  return `#${parts.join("")}`;
}

/** `rgb(r g b / alpha)` from a token, for translucent fills. */
function withAlpha(hex: string, alpha: number): string {
  return `rgb(${CHANNELS.map((index) => channel(hex, index)).join(" ")} / ${alpha})`;
}

export { mix, withAlpha };

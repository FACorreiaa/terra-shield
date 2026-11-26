import type { LucideIcon } from "lucide-react";
import type { ReactElement } from "react";

import { cn } from "@/lib/utils.ts";

import { controlButton } from "./controlStyles.ts";

interface LayerToggleProps {
  /** A stable key for the controls column. */
  id: string;
  /** What the layer is, e.g. "Satellite heat detections": the name, with the state as pressed. */
  label: string;
  icon: LucideIcon;
  pressed: boolean;
  /** Classes for the pressed look: the layer's own tint (MapControls.swift). */
  pressedClass: string;
  onToggle: () => void;
}

/** One map layer's switch in the controls column: a 44 px toggle button, tinted while on. */
function LayerToggle({
  label,
  icon: Glyph,
  pressed,
  pressedClass,
  onToggle,
}: Omit<LayerToggleProps, "id">): ReactElement {
  return (
    <button
      type="button"
      className={cn(controlButton, pressed && pressedClass)}
      aria-label={label}
      aria-pressed={pressed}
      title={`${pressed ? "Hide" : "Show"} ${label.toLowerCase()}`}
      onClick={onToggle}
    >
      <Glyph className="size-4" strokeWidth={2.25} aria-hidden />
    </button>
  );
}

export type { LayerToggleProps as LayerSwitch };
export { LayerToggle };

import type { LucideIcon, LucideProps } from "lucide-react";
import {
  Activity,
  BadgeCheck,
  Circle,
  Clock,
  Crosshair,
  CloudLightning,
  CloudRain,
  Diamond,
  Eye,
  Flame,
  Mountain,
  OctagonAlert,
  Snowflake,
  SunDim,
  Thermometer,
  ThermometerSnowflake,
  ThermometerSun,
  Tornado,
  Triangle,
  TriangleAlert,
  Waves,
  WavesArrowUp,
  Wind,
} from "lucide-react";
import type { ReactElement } from "react";
import { createElement } from "react";

/** Lucide components by the names `design/tokens.json` uses (`webIcon`). */
const icons: Readonly<Record<string, LucideIcon>> = {
  activity: Activity,
  "badge-check": BadgeCheck,
  circle: Circle,
  clock: Clock,
  crosshair: Crosshair,
  "cloud-lightning": CloudLightning,
  "cloud-rain": CloudRain,
  diamond: Diamond,
  eye: Eye,
  flame: Flame,
  mountain: Mountain,
  "octagon-alert": OctagonAlert,
  snowflake: Snowflake,
  "sun-dim": SunDim,
  thermometer: Thermometer,
  "thermometer-snowflake": ThermometerSnowflake,
  "thermometer-sun": ThermometerSun,
  tornado: Tornado,
  triangle: Triangle,
  "triangle-alert": TriangleAlert,
  waves: Waves,
  "waves-arrow-up": WavesArrowUp,
  wind: Wind,
};

interface IconProps extends LucideProps {
  /** A token icon name; an unknown name draws the generic warning triangle. */
  name: string;
}

/** A token-named Lucide icon. */
function Icon({ name, ...props }: IconProps): ReactElement {
  return createElement(icons[name] ?? TriangleAlert, props);
}

export { Icon };

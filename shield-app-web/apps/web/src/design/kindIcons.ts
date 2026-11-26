import type { EventKind } from "@shield/api-client";

/** The Lucide icon per kind (DESIGN.md §5); kind changes form, never colour. */
const KIND_ICONS: Readonly<Record<EventKind, string>> = {
  observed: "eye",
  warning: "badge-check",
  forecast: "clock",
  detection: "thermometer",
};

export { KIND_ICONS };

import type { LucideIcon } from "lucide-react";
import { Radar } from "lucide-react";

/**
 * The app's sections, in the order DESIGN.md §10 gives them (Map · Countries · Saved · Alerts ·
 * Sources). Only built sections are listed: each slice adds its own entry (Countries and Sources in
 * W5, Saved regions and Alerts in W9), and the sidebar, the sheet's header and the `1`–`5` keys all
 * read this list, so nothing points at a page that does not exist yet.
 */
interface Section {
  id: string;
  label: string;
  /** The route that opens it. */
  to: "/map";
  icon: LucideIcon;
  /** Its number key, `1`–`5`, fixed by its place in DESIGN's order. */
  key: string;
}

const SECTIONS: readonly Section[] = [{ id: "monitor", label: "Monitor", to: "/map", icon: Radar, key: "1" }];

export type { Section };
export { SECTIONS };

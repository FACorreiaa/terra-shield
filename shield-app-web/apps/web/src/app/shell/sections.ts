import type { LucideIcon } from "lucide-react";
import { Globe, Radar, RadioTower } from "lucide-react";

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
  to: "/map" | "/countries" | "/sources";
  icon: LucideIcon;
  /** Its number key, `1`–`5`, fixed by its place in DESIGN's order. */
  key: string;
}

/** Sources is pinned at the foot of the sidebar, apart from the others (DESIGN.md §10). */
const SOURCES_SECTION: Section = { id: "sources", label: "Sources", to: "/sources", icon: RadioTower, key: "5" };

const SECTIONS: readonly Section[] = [
  { id: "monitor", label: "Monitor", to: "/map", icon: Radar, key: "1" },
  { id: "countries", label: "Countries", to: "/countries", icon: Globe, key: "2" },
  SOURCES_SECTION,
];

/** The sections listed under the brand; Sources has its own row at the foot. */
const LISTED_SECTIONS = SECTIONS.filter((section) => section !== SOURCES_SECTION);

export type { Section };
export { LISTED_SECTIONS, SECTIONS, SOURCES_SECTION };

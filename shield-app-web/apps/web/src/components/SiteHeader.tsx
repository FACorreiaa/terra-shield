import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { Brand } from "@/app/shell/Brand.tsx";
import { keepSelection } from "@/lib/keepSelection.ts";

/** The site's own pages, for the header and the footer: the live map and the two lists beside it. */
const SITE_LINKS = [
  { to: "/map", label: "Live map" },
  { to: "/countries", label: "Countries" },
  { to: "/sources", label: "Sources" },
] as const;

const LINK =
  "flex min-h-11 items-center rounded-sm px-3 text-meta text-muted hover:text-ink focus-visible:outline-2 focus-visible:outline-accent aria-[current=page]:text-ink";

const PLACEMENT = {
  /** Over a scrolling page: stays at the top, content centred. */
  page: { header: "sticky top-0", row: "mx-auto max-w-6xl" },
  /** Over the desktop map: a bar above the sidebar and the map, edge to edge. */
  shell: { header: "shrink-0", row: "" },
} as const;

/**
 * The site's thin header (owner, 2026-11-26): the nav mark (DESIGN.md §11) and the site's pages, on
 * the landing page, a page that isn't there and over the desktop map. The phone map keeps its
 * capsule and the whole height.
 */
function SiteHeader({ placement = "page" }: { placement?: keyof typeof PLACEMENT }): ReactElement {
  const { header, row } = PLACEMENT[placement];
  return (
    <header
      className={`${header} z-20 border-b border-hairline material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none`}
    >
      <div className={`${row} flex items-center justify-between gap-2 px-4`}>
        <Brand compact="narrow" />
        <nav aria-label="Site">
          <ul className="flex items-center">
            {SITE_LINKS.map(({ to, label }) => (
              <li key={to}>
                <Link to={to} search={keepSelection} className={LINK}>
                  {label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </header>
  );
}

export { LINK as SITE_LINK, SITE_LINKS, SiteHeader };

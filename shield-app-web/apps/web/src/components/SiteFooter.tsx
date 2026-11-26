import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

import { Brand } from "@/app/shell/Brand.tsx";

import { DisclaimerBanner } from "./DisclaimerBanner.tsx";
import { SITE_LINK, SITE_LINKS } from "./SiteHeader.tsx";

/** The web footer (DESIGN.md §2): the mark and promise, the site's pages, and the disclaimer last. */
function SiteFooter(): ReactElement {
  return (
    <footer className="border-t border-hairline bg-canvas">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-10 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col gap-2">
          <Brand />
          <p className="max-w-xs text-meta text-muted">Global hazard intelligence. Official sources, live.</p>
        </div>
        <nav aria-label="Footer">
          <ul className="-mx-3 flex flex-col sm:mx-0 sm:flex-row">
            {SITE_LINKS.map(({ to, label }) => (
              <li key={to}>
                <Link to={to} className={SITE_LINK}>
                  {label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="border-t border-hairline">
        <DisclaimerBanner variant="footer" className="mx-auto max-w-6xl px-4 py-4" />
      </div>
    </footer>
  );
}

export { SiteFooter };

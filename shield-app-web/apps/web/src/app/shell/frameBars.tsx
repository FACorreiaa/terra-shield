import type { ReactElement } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";
import { SiteHeader } from "@/components/SiteHeader.tsx";

/** The site's header over the desktop map, above the sidebar and the map (owner, 2026-11-26). */
function ShellHeader(): ReactElement {
  return <SiteHeader placement="shell" />;
}

/** The desktop map's disclaimer strip, under the map. */
function ShellFooter(): ReactElement {
  return (
    <footer className="border-t border-hairline px-4 py-2">
      <DisclaimerBanner variant="footer" />
    </footer>
  );
}

export { ShellFooter, ShellHeader };

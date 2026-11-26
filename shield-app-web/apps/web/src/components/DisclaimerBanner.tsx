import type { ReactElement } from "react";

import { cn } from "@/lib/utils.ts";

/** The one copy of the permanent disclaimer (DESIGN.md §2, DECISIONS #12). */
const DISCLAIMER = "Aegis Terra does not replace your national emergency service. Always follow official instructions.";

interface DisclaimerBannerProps {
  /** `inline` sits in content (Sources, landing); `footer` closes an event or a page. */
  variant: "inline" | "footer";
  className?: string;
}

function DisclaimerBanner({ variant, className }: DisclaimerBannerProps): ReactElement {
  return (
    <p
      className={cn(
        "text-meta text-muted",
        variant === "inline" && "rounded-md border border-hairline bg-panel px-4 py-3 text-ink",
        className,
      )}
    >
      {DISCLAIMER}
    </p>
  );
}

export { DISCLAIMER, DisclaimerBanner };

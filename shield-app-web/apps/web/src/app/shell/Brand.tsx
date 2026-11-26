import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

/** The nav mark and `AEGIS TERRA` micro-label, linking home (DESIGN.md §10 shells). */
/** The nav mark and wordmark; `compact` keeps only the mark (the name stays for screen readers). */
function Brand({ compact = false }: { compact?: boolean }): ReactElement {
  return (
    <Link
      to="/"
      className="flex h-11 shrink-0 items-center gap-2.5 rounded-sm pr-2 whitespace-nowrap focus-visible:outline-2 focus-visible:outline-accent"
    >
      <img src="/favicon.png" alt="" className="size-7" />
      <span className={compact ? "sr-only" : "text-micro tracking-[0.08em] text-ink uppercase"}>Aegis Terra</span>
    </Link>
  );
}

export { Brand };

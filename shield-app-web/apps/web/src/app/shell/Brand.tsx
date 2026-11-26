import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

const WORDMARK = "text-micro tracking-[0.08em] text-ink uppercase";

/**
 * The nav mark and `AEGIS TERRA` micro-label, linking home (DESIGN.md §10 shells, §11). `compact`
 * keeps only the mark, always or on a phone (`"narrow"`); the name stays for screen readers.
 */
function Brand({ compact = false }: { compact?: boolean | "narrow" }): ReactElement {
  const words = { true: "sr-only", false: WORDMARK, narrow: `sr-only sm:not-sr-only ${WORDMARK}` }[String(compact)];
  return (
    <Link
      to="/"
      className="flex h-11 shrink-0 items-center gap-2.5 rounded-sm pr-2 whitespace-nowrap focus-visible:outline-2 focus-visible:outline-accent"
    >
      <img src="/favicon.png" alt="" className="size-7" />
      <span className={words}>Aegis Terra</span>
    </Link>
  );
}

export { Brand };

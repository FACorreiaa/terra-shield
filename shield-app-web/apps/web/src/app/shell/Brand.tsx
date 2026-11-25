import { Link } from "@tanstack/react-router";
import type { ReactElement } from "react";

/** The nav mark and `AEGIS TERRA` micro-label, linking home (DESIGN.md §10 shells). */
function Brand(): ReactElement {
  return (
    <Link
      to="/"
      className="flex h-11 items-center gap-2.5 rounded-sm pr-2 focus-visible:outline-2 focus-visible:outline-accent"
    >
      <img src="/favicon.png" alt="" className="size-7" />
      <span className="text-micro tracking-[0.08em] text-ink uppercase">Aegis Terra</span>
    </Link>
  );
}

export { Brand };

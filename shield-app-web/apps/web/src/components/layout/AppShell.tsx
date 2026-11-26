import { Link } from "@tanstack/react-router";
import type { ReactElement, ReactNode } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";

/** Header, content and the permanent disclaimer (DESIGN.md §2, DECISIONS #12). W3 replaces it with the shells. */
function AppShell({ children }: { children: ReactNode }): ReactElement {
  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-3 border-b border-hairline bg-canvas px-4 py-2">
        <Link
          to="/"
          className="flex items-center gap-3 rounded-sm focus-visible:outline-2 focus-visible:outline-accent"
        >
          <img src="/favicon.png" alt="" className="size-7" />
          <span className="text-micro tracking-[0.08em] text-ink uppercase">Aegis Terra</span>
        </Link>
      </header>
      <main className="relative min-h-0 flex-1">{children}</main>
      <footer className="border-t border-hairline bg-canvas px-4 py-2">
        <DisclaimerBanner variant="footer" />
      </footer>
    </div>
  );
}

export { AppShell };

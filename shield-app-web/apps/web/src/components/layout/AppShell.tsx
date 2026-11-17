import type { ReactElement, ReactNode } from "react";

/** Header, content and the permanent disclaimer (DESIGN.md §2, DECISIONS #12). */
function AppShell({ children }: { children: ReactNode }): ReactElement {
  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-3 border-b border-ink/8 bg-canvas px-4 py-2">
        <img src="/favicon.png" alt="" className="size-7" />
        <span className="text-headline tracking-wide text-ink">Aegis Terra</span>
        <span className="text-micro tracking-[0.06em] text-faint uppercase">Earth monitor</span>
      </header>
      <main className="relative min-h-0 flex-1">{children}</main>
      <footer className="border-t border-ink/8 bg-canvas px-4 py-2 text-meta text-muted">
        Aegis Terra does not replace your national emergency service. Always follow official instructions.
      </footer>
    </div>
  );
}

export { AppShell };

import type { ReactElement, ReactNode } from "react";

/** One part of a country's overview, under its small caps title. */
function Section({ title, children }: { title: string; children: ReactNode }): ReactElement {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-micro tracking-[0.08em] text-muted uppercase">{title}</h3>
      {children}
    </section>
  );
}

export { Section };

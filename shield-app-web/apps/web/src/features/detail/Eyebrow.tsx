import type { ReactElement, ReactNode } from "react";

/** A section's small caps label in the detail: "SEVERITY · AEGIS / SOURCE", "SOURCE", "TIMELINE". */
function Eyebrow({ children }: { children: ReactNode }): ReactElement {
  return <h3 className="text-micro tracking-[0.08em] text-faint uppercase">{children}</h3>;
}

export { Eyebrow };

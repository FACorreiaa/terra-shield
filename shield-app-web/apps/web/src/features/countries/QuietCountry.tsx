import type { ReactElement, ReactNode } from "react";

import { EmptyState } from "@/components/EmptyState.tsx";

/** No active events (DESIGN.md §10.2 states): the Sentinel, then the coverage notice says what isn't covered. */
function QuietCountry({ name, children }: { name: string; children: ReactNode }): ReactElement {
  return (
    <>
      <EmptyState>
        <p className="text-body text-ink">{`No active events in ${name} from covered sources.`}</p>
      </EmptyState>
      {children}
    </>
  );
}

export { QuietCountry };

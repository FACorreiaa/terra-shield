import type { ReactElement, ReactNode } from "react";

/**
 * An empty list with the Sentinel beside (or, when narrow, above) its words (DESIGN.md §11): never on the map canvas, at
 * least 120 px tall, once per screen. The words say what isn't covered, never "no threats".
 */
function EmptyState({ children }: { children: ReactNode }): ReactElement {
  return (
    // Beside the words where there is room; above them in a narrow column such as the sidebar.
    <div className="@container px-4 py-4">
      <div className="flex min-h-30 flex-col items-start gap-3 @xs:flex-row @xs:items-center @xs:gap-4">
        <img src="/brand/sentinel.webp" alt="" width={96} height={128} className="h-32 w-24 shrink-0 object-contain" />
        <div className="flex min-w-0 flex-col gap-2">{children}</div>
      </div>
    </div>
  );
}

export { EmptyState };

import type { Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { Icon } from "@/design/icons.ts";
import { hazardTypeTokens, severityTokens } from "@/design/tokens.gen.ts";
import type { MatrixCell, MatrixRow } from "@/domain/countries.ts";
import { typeLabel } from "@/domain/taxonomy.ts";
import { cn } from "@/lib/utils.ts";

import { Section } from "./Section.tsx";

/** The severity's tint at `alpha.matrixTint` (25%) over `raised`, and its colour for the column's shape. */
const TINT: Readonly<Record<Severity, string>> = {
  minor: "bg-severity-minor/25",
  moderate: "bg-severity-moderate/25",
  severe: "bg-severity-severe/25",
  extreme: "bg-severity-extreme/25",
};
const SHAPE: Readonly<Record<Severity, string>> = {
  minor: "text-severity-minor",
  moderate: "text-severity-moderate",
  severe: "text-severity-severe",
  extreme: "text-severity-extreme",
};
/** A severity's word cut to this many letters where the full word has no room. */
const SHORT = 3;
/** Where a type's name and four cells share a line (from 20rem); narrower, the name sits above. */
const GRID = "grid grid-cols-4 gap-1 @xs:grid-cols-[minmax(6rem,1.4fr)_repeat(4,minmax(2.75rem,1fr))]";

interface MatrixProps {
  rows: readonly MatrixRow[];
  chosen: MatrixCell | undefined;
  onChoose: (cell: MatrixCell | undefined) => void;
}

function Header(): ReactElement {
  return (
    <>
      <span aria-hidden className="hidden @xs:block" />
      {SEVERITY_ORDER.map((severity) => {
        const token = severityTokens[severity];
        return (
          <span key={severity} aria-hidden className="flex flex-col items-center gap-0.5 pb-1">
            <Icon name={token.icon} className={cn("size-3", SHAPE[severity])} fill="currentColor" strokeWidth={2.5} />
            {/* The full word where it fits, its first three letters where it doesn't (the sidebar). */}
            <span className="text-micro text-muted @xs:hidden">{token.label.slice(0, SHORT)}</span>
            <span className="hidden text-micro text-muted @xs:inline">{token.label}</span>
          </span>
        );
      })}
    </>
  );
}

interface CellProps {
  row: MatrixRow;
  severity: Severity;
  chosen: boolean;
  onChoose: (cell: MatrixCell | undefined) => void;
}

function Cell({ row, severity, chosen, onChoose }: CellProps): ReactElement {
  const count = row.counts[severity] ?? 0;
  const words = `${typeLabel(row.type)}, ${severityTokens[severity].label}`;
  const choose = useCallback(() => {
    onChoose(chosen ? undefined : { type: row.type, severity });
  }, [onChoose, chosen, row.type, severity]);
  const box = "relative flex min-h-11 items-center justify-center rounded-sm text-meta tabular-nums";
  if (count === 0) {
    return (
      <span className={cn(box, "border border-ink/8 bg-raised text-muted")}>
        <span aria-hidden>–</span>
        <span className="sr-only">{`${words}: none`}</span>
      </span>
    );
  }
  return (
    <button
      type="button"
      onClick={choose}
      aria-pressed={chosen}
      aria-label={`${words}: ${count}`}
      title={chosen ? "Shows every event again" : "Lists only these events"}
      className={cn(
        box,
        "bg-raised font-semibold text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
        chosen ? "border-2 border-accent" : "border border-ink/8 hover:border-ink/30",
      )}
    >
      {/* The tint over opaque `raised`, inside the border. */}
      <span aria-hidden className={cn("absolute inset-0 rounded-sm", TINT[severity])} />
      <span className="relative">{count}</span>
    </button>
  );
}

/**
 * A country's active events by hazard type and severity (DESIGN.md §10.2, TypeSeverityMatrix.swift):
 * rows are the types present, columns minor → extreme with their shape and word, never colour
 * alone. A cell with events is tinted by its severity and filters the list; the chosen one is
 * outlined in accent.
 */
function TypeSeverityMatrix({ rows, chosen, onChoose }: MatrixProps): ReactElement {
  return (
    <div className="@container">
      <div className={GRID}>
        <Header />
        {rows.map((row) => {
          const token = hazardTypeTokens[row.type];
          return (
            <div key={row.type} className="contents">
              <span className="col-span-4 flex items-center gap-1.5 pt-1 text-meta text-ink @xs:col-span-1 @xs:pt-0">
                <Icon name={token.icon} aria-hidden className="size-3.5 shrink-0 text-muted" />
                <span className="min-w-0">{typeLabel(row.type)}</span>
              </span>
              {SEVERITY_ORDER.map((severity) => (
                <Cell
                  key={severity}
                  row={row}
                  severity={severity}
                  chosen={chosen?.type === row.type && chosen.severity === severity}
                  onChoose={onChoose}
                />
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}

/** The matrix under its title (DESIGN.md §10.2 item 3). */
function CountryMatrix(props: MatrixProps): ReactElement {
  return (
    <Section title="By type and severity">
      <TypeSeverityMatrix rows={props.rows} chosen={props.chosen} onChoose={props.onChoose} />
    </Section>
  );
}

export { CountryMatrix, TypeSeverityMatrix };

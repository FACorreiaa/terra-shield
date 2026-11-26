import type { HazardEvent, Source } from "@shield/api-client";
import type { ReactElement } from "react";
import { useCallback, useState } from "react";

import { notableChanges } from "@/domain/changes.ts";
import type { CountrySummary, MatrixCell } from "@/domain/countries.ts";
import { countIn, matrix } from "@/domain/countries.ts";

import { CountryEvents } from "./CountryEvents.tsx";
import { CoverageNotice } from "./CoverageNotice.tsx";
import { QuietCountry } from "./QuietCountry.tsx";
import { RecentChanges } from "./RecentChanges.tsx";
import { CountryMatrix } from "./TypeSeverityMatrix.tsx";

interface CountryBodyProps {
  summary: CountrySummary;
  sources: readonly Source[] | undefined;
  selectedId: string | undefined;
  onSelect: (event: HazardEvent) => void;
  now: Date;
}

/** The chosen matrix cell; one that emptied since it was chosen would leave an unexplained empty list. */
function useCell(summary: CountrySummary): [MatrixCell | undefined, (cell: MatrixCell | undefined) => void] {
  const [cell, setCell] = useState<MatrixCell>();
  if (cell !== undefined && countIn(summary, cell) === 0) {
    setCell(undefined);
  }
  return [cell, setCell];
}

/**
 * A loaded overview after its numbers (DESIGN.md §10.2 items 3–6): the matrix, coverage, recent
 * changes and the events; with none active, the Sentinel and the coverage notice.
 */
function CountryBody({ summary, sources, selectedId, onSelect, now }: CountryBodyProps): ReactElement {
  const [cell, setCell] = useCell(summary);
  const clearCell = useCallback(() => {
    setCell(undefined);
  }, [setCell]);
  const { name } = summary.country;
  const coverage = <CoverageNotice countryName={name} coverage={summary.coverage} sources={sources} />;
  if (summary.country.activeEvents === 0) {
    return <QuietCountry name={name}>{coverage}</QuietCountry>;
  }
  return (
    <>
      <CountryMatrix rows={matrix(summary)} chosen={cell} onChoose={setCell} />
      {coverage}
      <RecentChanges
        changes={notableChanges(summary.recentChanges)}
        events={summary.events}
        now={now}
        onSelect={onSelect}
      />
      <CountryEvents
        summary={summary}
        cell={cell}
        onClearCell={clearCell}
        selectedId={selectedId}
        onSelect={onSelect}
        now={now}
      />
    </>
  );
}

export { CountryBody };

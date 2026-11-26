import type { Source } from "@shield/api-client";
import { BadgeCheck, TriangleAlert } from "lucide-react";
import type { ReactElement } from "react";

import type { CountrySummary } from "@/domain/countries.ts";
import { cn } from "@/lib/utils.ts";

const LIST = new Intl.ListFormat("en", { type: "conjunction" });

interface CoverageNoticeProps {
  countryName: string;
  coverage: CountrySummary["coverage"];
  /** From `/v1/sources`, for names; an id shows until they load. */
  sources: readonly Source[] | undefined;
}

/**
 * Who covers a country, and what that leaves out (DESIGN.md §10.2, ADR 7). Always shown: a quiet
 * map is only reassuring if you know who is watching. It says only what the API knows, so it never
 * names a missing national service.
 */
function CoverageNotice({ countryName, coverage, sources }: CoverageNoticeProps): ReactElement {
  const names = coverage.sources.map((id) => sources?.find((source) => source.id === id)?.name ?? id.toUpperCase());
  const covered = coverage.officialWarnings;
  return (
    <section
      aria-label="Coverage"
      className={cn(
        "flex flex-col gap-2 rounded-md border bg-ink/4 p-3 text-meta",
        covered ? "border-ink/8" : "border-warning/60",
      )}
    >
      <p className="flex items-start gap-2 text-ink">
        {covered ? (
          <BadgeCheck aria-hidden className="mt-0.5 size-4 shrink-0 text-muted" />
        ) : (
          <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warning" />
        )}
        {covered
          ? `Official warnings cover ${countryName}.`
          : `Limited coverage: no official warning service for ${countryName} feeds Aegis Terra, so warnings may be missing.`}
      </p>
      {names.length === 0 ? undefined : <p className="text-muted">{`Sources: ${LIST.format(names)}.`}</p>}
      <p className="text-muted">Absence of data is not safety.</p>
    </section>
  );
}

export { CoverageNotice };

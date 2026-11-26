import type { HazardEvent, Source } from "@shield/api-client";
import type { ReactElement } from "react";

import { OfficialSourceButton, SourceBadge } from "@/features/sources/SourceBadge.tsx";

import { DetailFooter } from "./DetailFooter.tsx";
import type { DetailState } from "./detailState.ts";
import { officialUrl } from "./officialSource.ts";
import { RevisionTimeline } from "./RevisionTimeline.tsx";

interface DetailRecordProps {
  event: HazardEvent;
  state: DetailState;
  source: Source | undefined;
  now: Date;
}

function Divider(): ReactElement {
  return <hr className="border-hairline" />;
}

/** Who said so, with the way to their record; gone, the event's own page is likely dead, so the source's home. */
function SourceBlock({ event, state, source, now }: DetailRecordProps): ReactElement | undefined {
  const url = officialUrl(event, source, state.kind === "gone");
  if (source !== undefined) {
    const checked = source.lastSuccessAt === undefined ? undefined : new Date(source.lastSuccessAt);
    return <SourceBadge source={source} url={url} checkedAt={checked} now={now} />;
  }
  // No source record yet: the official record still opens.
  return url === undefined ? undefined : <OfficialSourceButton url={url} />;
}

/** The source, what changed, then the disclaimer and Share (DESIGN.md §10.3 items 6–8). */
function DetailRecord({ event, state, source, now }: DetailRecordProps): ReactElement {
  return (
    <>
      <Divider />
      <SourceBlock event={event} state={state} source={source} now={now} />
      <Divider />
      <RevisionTimeline state={state} now={now} />
      <Divider />
      <DetailFooter eventId={event.id} />
    </>
  );
}

export { DetailRecord };

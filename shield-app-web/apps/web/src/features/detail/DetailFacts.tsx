import type { HazardEvent } from "@shield/api-client";
import type { ReactElement } from "react";

import { keyFigures } from "@/domain/eventFacts.ts";

import { KeyFigures } from "./KeyFigures.tsx";
import { KindAndTimes } from "./KindAndTimes.tsx";
import { SeverityBlock } from "./SeverityBlock.tsx";

/** Both severities and why, kind and times, the source's own summary, key figures (DESIGN.md §10.3 items 2–4). */
function DetailFacts({ event, gone, now }: { event: HazardEvent; gone: boolean; now: Date }): ReactElement {
  return (
    <>
      <SeverityBlock event={event} />
      <KindAndTimes event={event} gone={gone} now={now} />
      {event.summary === undefined || event.summary === "" ? undefined : (
        <p className="text-body text-ink">{event.summary}</p>
      )}
      <KeyFigures figures={keyFigures(event)} />
    </>
  );
}

export { DetailFacts };

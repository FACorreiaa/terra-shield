import type { HazardEvent, Source } from "@shield/api-client";
import type { ReactElement } from "react";

import type { EventShape } from "@/domain/geo.ts";

import { GoneBanner, StaleBanner } from "./DetailBanner.tsx";
import { DetailFacts } from "./DetailFacts.tsx";
import { DetailHeader } from "./DetailHeader.tsx";
import { DetailRecord } from "./DetailRecord.tsx";
import type { DetailState } from "./detailState.ts";
import { EventMapBlock } from "./EventMapSnippet.tsx";

interface EventDetailViewProps {
  event: HazardEvent;
  state: DetailState;
  /** From `/v1/sources`, else the detail's own source record. */
  source: Source | undefined;
  /** The event's source has missed three polls. */
  stale: boolean;
  shape: EventShape | undefined;
  now: Date;
  onClose: () => void;
  onShowOnMap: () => void;
}

/**
 * One event in full (DESIGN.md §10.3, EventDetailView.swift): what and where, both severities and
 * why, kind and times, key figures, the area on a map, who said so with the way to their record,
 * and what changed. The inspector shows it on a wide screen, the sheet from half height on a phone.
 */
function EventDetailView(props: EventDetailViewProps): ReactElement {
  const { event, state, source, stale, shape, now, onClose, onShowOnMap } = props;
  const gone = state.kind === "gone";
  const resolved = event.status === "resolved";
  return (
    <article aria-label={event.title} className="flex flex-col gap-5 p-4">
      <DetailHeader event={event} stale={stale} gone={gone} onClose={onClose} />
      {gone ? <GoneBanner sourceName={source?.name} /> : undefined}
      {!gone && stale && !resolved ? (
        <StaleBanner lastUpdate={new Date(source?.lastSuccessAt ?? event.fetchedAt)} now={now} />
      ) : undefined}
      <DetailFacts event={event} gone={gone} now={now} />
      <EventMapBlock event={event} shape={shape} faded={resolved || gone || stale} onShowOnMap={onShowOnMap} />
      <DetailRecord event={event} state={state} source={source} now={now} />
    </article>
  );
}

export { EventDetailView };

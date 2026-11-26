import { useQueryClient } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { CONFLICT } from "@/domain/taxonomy.ts";
import { DetailPlaceholder } from "@/features/detail/DetailPlaceholder.tsx";
import { EventDetailView } from "@/features/detail/EventDetailView.tsx";
import { sourceOf } from "@/features/detail/officialSource.ts";
import { SelectedAircraft } from "@/features/flights/SelectedAircraft.tsx";

import { useLiveMap } from "./liveMapContext.ts";
import { useFocusOnOpen } from "./useDetailFocus.ts";
import { useShowOnMap } from "./useShowOnMap.ts";

function SelectedEvent(): ReactElement | undefined {
  const { selectedId, selected, detail, selectedShape, sources, staleSources, select, now } = useLiveMap();
  const close = useCallback(() => {
    select(undefined);
  }, [select]);
  const client = useQueryClient();
  const retry = useCallback(() => {
    void client.refetchQueries({ queryKey: ["event", selectedId] });
  }, [client, selectedId]);
  const showOnMap = useShowOnMap();
  if (detail === undefined) {
    return undefined;
  }
  if (selected === undefined) {
    return <DetailPlaceholder state={detail} onClose={close} onRetry={retry} />;
  }
  const source = sourceOf(selected, sources, detail);
  return (
    <EventDetailView
      event={selected}
      state={detail}
      source={source}
      // A conflict is a monthly estimate, never stale (EventRow.swift).
      stale={selected.type !== CONFLICT && staleSources.has(selected.source)}
      shape={selectedShape}
      now={now}
      onClose={close}
      onShowOnMap={showOnMap}
    />
  );
}

/** The selection's detail, an event's or an aircraft's, for the inspector and the sheet alike. */
function SelectedDetail(): ReactElement | undefined {
  const { selectedId, aircraftHex, selectAircraft } = useLiveMap();
  useFocusOnOpen(selectedId ?? aircraftHex);
  const closeAircraft = useCallback(() => {
    selectAircraft(undefined);
  }, [selectAircraft]);
  return aircraftHex === undefined ? <SelectedEvent /> : <SelectedAircraft onClose={closeAircraft} />;
}

export { SelectedDetail };

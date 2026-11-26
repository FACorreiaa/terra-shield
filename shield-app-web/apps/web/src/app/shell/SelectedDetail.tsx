import { useQueryClient } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { useCallback } from "react";

import { DetailPlaceholder } from "@/features/detail/DetailPlaceholder.tsx";
import { EventDetailView } from "@/features/detail/EventDetailView.tsx";
import { sourceOf } from "@/features/detail/officialSource.ts";

import { useLiveMap } from "./liveMapContext.ts";
import { useFocusOnOpen } from "./useDetailFocus.ts";
import { useShowOnMap } from "./useShowOnMap.ts";

/** The selection's detail, for the inspector and the sheet alike; nothing with nothing selected. */
function SelectedDetail(): ReactElement | undefined {
  const { selectedId, selected, detail, selectedShape, sources, staleSources, select, now } = useLiveMap();
  const close = useCallback(() => {
    select(undefined);
  }, [select]);
  const client = useQueryClient();
  const retry = useCallback(() => {
    void client.refetchQueries({ queryKey: ["event", selectedId] });
  }, [client, selectedId]);
  useFocusOnOpen(selectedId);
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
      stale={staleSources.has(selected.source)}
      shape={selectedShape}
      now={now}
      onClose={close}
      onShowOnMap={showOnMap}
    />
  );
}

export { SelectedDetail };

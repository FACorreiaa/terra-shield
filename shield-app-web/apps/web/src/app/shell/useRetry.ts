import { useCallback } from "react";

import { useLiveMap } from "./liveMapContext.ts";

/** Loads the events again now. */
function useRetry(): () => void {
  const { refetch } = useLiveMap().events;
  return useCallback(() => {
    void refetch();
  }, [refetch]);
}

export { useRetry };

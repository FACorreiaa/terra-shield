import { useEffect } from "react";

/**
 * Refreshes once when the page opens (SourcesView.swift `.task`), whatever the cache holds: a
 * query's `staleTime: 0` alone doesn't refetch when another observer keeps it, so ask outright.
 */
function useRefreshOnOpen(refetch: (options: { cancelRefetch: boolean }) => Promise<unknown>): void {
  useEffect(() => {
    // Joins a fetch already under way (the page's first load) rather than aborting it.
    void refetch({ cancelRefetch: false });
  }, [refetch]);
}

export { useRefreshOnOpen };

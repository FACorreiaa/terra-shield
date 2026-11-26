import type { UseQueryResult } from "@tanstack/react-query";
import { useState } from "react";

/**
 * Counts the times a map layer was turned on. Each time starts afresh, as the apps' layers do: an
 * answer from before it was last turned off (another view, an old error) never shows again.
 */
function useSession(shown: boolean): number {
  const [state, setState] = useState({ shown, session: 0 });
  if (state.shown === shown) {
    return state.session;
  }
  // The new session counts from this very render, so no hook below asks with the old one's key.
  const next = { shown, session: shown ? state.session + 1 : state.session };
  setState(next);
  return next.session;
}

interface Kept<Data> {
  session: number;
  data: Data;
}

/**
 * The latest answer of this session that arrived; kept through a failed load for a new view. An
 * answer is kept with its session and given back only in that session.
 */
function useLastGood<Data>(query: UseQueryResult<Data>, shown: boolean, session: number): Data | undefined {
  const [last, setLast] = useState<Kept<Data>>();
  const fresh = shown && query.isSuccess && !query.isPlaceholderData;
  if (fresh && (last?.session !== session || last.data !== query.data)) {
    setLast({ session, data: query.data });
  }
  if (!shown && last !== undefined) {
    setLast(undefined);
  }
  return shown && last?.session === session ? last.data : undefined;
}

/**
 * Whether a layer query's previous answer may stand in across a pan: only one from this session
 * (at `queryKey[1]`), never one from before the layer was last turned on.
 */
function sameSession(previousQuery: { queryKey: readonly unknown[] } | undefined, session: number): boolean {
  return previousQuery?.queryKey[1] === session;
}

export { sameSession, useLastGood, useSession };

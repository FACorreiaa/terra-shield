import type { Source } from "@shield/api-client";
import { createContext } from "react";

/** What a list row needs from `/v1/sources`: each source's name, and which are stale. */
interface SourceLookup {
  byId: ReadonlyMap<string, Source>;
  /** Sources that have missed three polls (DECISIONS #8): their rows fade like their markers. */
  stale: ReadonlySet<string>;
}

const SourceLookupContext = createContext<SourceLookup>({ byId: new Map(), stale: new Set() });

export type { SourceLookup };
export { SourceLookupContext };

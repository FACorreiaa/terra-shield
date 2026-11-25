import { useMemo } from "react";

import type { StatCell } from "@/domain/statStrip.ts";
import { liveMapCells } from "@/domain/statStrip.ts";

import { useLiveMap } from "./liveMapContext.ts";

/** The strip's cells for what the map shows; resolved events never count (StatStrip.swift). */
function useCells(): readonly StatCell[] {
  const { shown, query } = useLiveMap();
  return useMemo(() => liveMapCells(shown, query.includeResolved), [shown, query.includeResolved]);
}

export { useCells };

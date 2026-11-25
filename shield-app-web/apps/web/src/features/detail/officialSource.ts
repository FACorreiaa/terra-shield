import type { HazardEvent, Source } from "@shield/api-client";

import type { DetailState } from "./detailState.ts";

/** The event's source record: from `/v1/sources`, else the detail's own copy. */
function sourceOf(
  event: Pick<HazardEvent, "source">,
  sources: readonly Source[] | undefined,
  detail: DetailState | undefined,
): Source | undefined {
  const listed = sources?.find((source) => source.id === event.source);
  return listed ?? (detail?.kind === "loaded" ? detail.detail.sourceInfo : undefined);
}

/**
 * Where "Open official source" (and `o`) goes: the event's own page, else the source's home. Gone,
 * the event's page is likely dead, so only the source's home, as the gone banner says.
 */
function officialUrl(
  event: Pick<HazardEvent, "url">,
  source: Pick<Source, "url"> | undefined,
  gone: boolean,
): string | undefined {
  const own = event.url === "" || gone ? undefined : event.url;
  return own ?? source?.url;
}

export { officialUrl, sourceOf };

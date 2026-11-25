import { describe, expect, it } from "vite-plus/test";

import { sortedForDisplay } from "./sort.ts";
import { sampleEvent, sampleEvents } from "./testEvents.ts";

describe(sortedForDisplay, () => {
  it("orders by severity, then recency, resolved last", () => {
    const sorted = sortedForDisplay(sampleEvents());
    expect(sorted[0]?.severity).toBe("extreme");
    expect(sorted.filter((event) => event.severity === "severe").map((event) => event.title)).toStrictEqual([
      "M6.1 earthquake off Coquimbo",
      "Flood warning, Emilia-Romagna",
    ]);
    expect(sorted.at(-1)?.status).toBe("resolved");
  });

  it("puts a resolved event after every active one, whatever its severity", () => {
    const resolvedExtreme = { ...sampleEvent(0), status: "resolved" as const };
    const minor = sampleEvent(5);
    expect(sortedForDisplay([resolvedExtreme, minor]).map((event) => event.id)).toStrictEqual([
      minor.id,
      resolvedExtreme.id,
    ]);
  });

  it("leaves its input as it was", () => {
    const events = sampleEvents();
    const ids = events.map((event) => event.id);
    sortedForDisplay(events);
    expect(events.map((event) => event.id)).toStrictEqual(ids);
  });
});

import { describe, expect, it } from "vite-plus/test";

import { createShieldClient, hazardType } from "./index.ts";

const eventList = {
  events: [
    {
      id: "6f1c2a7e-0000-4000-8000-000000000001",
      source: "usgs",
      type: "earthquake",
      kind: "observed",
      status: "active",
      severity: "severe",
      sourceSeverity: "M 6.1 mww, PAGER yellow",
      title: "M 6.1 - 38 km W of Coquimbo, Chile",
      centroid: { latitude: -30.2, longitude: -71.6 },
      countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38.4 }],
      observedAt: "2026-11-16T13:26:00Z",
      updatedAt: "2026-11-16T13:34:00.123456789Z",
      fetchedAt: "2026-11-16T13:35:01.5Z",
      attrs: {},
    },
  ],
  clusters: [],
  truncated: false,
  generatedAt: "2026-11-16T13:35:02Z",
};

/** A fetch that records each URL and answers with `body` and `status`. No network. */
function stubFetch(seen: string[], body: unknown, status = 200): typeof fetch {
  return (input) => {
    seen.push(input instanceof Request ? input.url : String(input));
    return Promise.resolve(Response.json(body, { status }));
  };
}

describe(createShieldClient, () => {
  it("sends filters as query parameters and returns typed events", async () => {
    expect.assertions(3);
    const seen: string[] = [];
    const client = createShieldClient("http://shield.test", stubFetch(seen, eventList));
    const { data, error } = await client.GET("/v1/events", {
      params: { query: { zoom: 2, severity: "severe,extreme" } },
    });
    expect(error).toBeUndefined();
    // oxlint-disable-next-line vitest/no-conditional-in-test -- narrows the events/clusters union
    expect(data !== undefined && "events" in data ? data.events[0]?.centroid.latitude : undefined).toBe(-30.2);
    expect(seen[0]).toBe("http://shield.test/v1/events?zoom=2&severity=severe%2Cextreme");
  });

  it("returns the error envelope on failure", async () => {
    expect.assertions(2);
    const envelope = { error: true, code: "unavailable", reason: "database down" };
    const client = createShieldClient("http://shield.test", stubFetch([], envelope, 503));
    const { data, error } = await client.GET("/v1/events", { params: { query: { zoom: 2 } } });
    expect(data).toBeUndefined();
    expect(error?.reason).toBe("database down");
  });
});

describe(hazardType, () => {
  it("keeps known types and maps unknown ones to other", () => {
    expect(hazardType("volcano")).toBe("volcano");
    expect(hazardType("landslide")).toBe("other");
  });
});

import type { EventList, HazardEvent } from "@shield/api-client";
import type { UseQueryResult } from "@tanstack/react-query";
import { QueryClient } from "@tanstack/react-query";
import type { Mock } from "vite-plus/test";
import { describe, expect, it, vi } from "vite-plus/test";

import { DEFAULT_QUERY } from "@/domain/filters.ts";
import { makeEvent } from "@/domain/testEvents.ts";
import type { View } from "@/domain/viewport.ts";
import { eventsKey } from "@/features/events/useEvents.ts";
import type { LiveChange } from "@/lib/live/liveChange.ts";

import { applyUpdate } from "./useLiveChanges.ts";

const VIEW: View = { zoom: 7, bbox: "-25,34,45,72" };
const ELSEWHERE: View = { zoom: 7, bbox: "100,-50,170,0" };
const LOADED_AT = 1_700_000_000_000;

function recent(id: string): HazardEvent {
  const at = new Date().toISOString();
  return makeEvent({ id, centroid: { latitude: 48, longitude: 2 }, observedAt: at, updatedAt: at });
}

function list(events: HazardEvent[]): EventList {
  return { events, clusters: [], truncated: false, generatedAt: new Date().toISOString() };
}

type Schedule = (after: number, refetch: () => unknown) => void;

interface Harness {
  client: QueryClient;
  schedule: Mock<Schedule>;
  arrivals: Mock<(event: HazardEvent, animate: boolean) => void>;
  stream: { ready: boolean };
  apply: (change: LiveChange, selectedId?: string) => void;
}

function harness(): Harness {
  const client = new QueryClient();
  const schedule = vi.fn<Schedule>();
  const arrivals = vi.fn<(event: HazardEvent, animate: boolean) => void>();
  const stream = { ready: false };
  // Only `refetch` is read, and only handed to the reloader.
  // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- a stub of the one field used
  const events = { refetch: vi.fn<() => Promise<void>>() } as unknown as UseQueryResult<EventList>;
  const apply = (change: LiveChange, selectedId?: string): void => {
    applyUpdate(
      { change, cursor: "42" },
      {
        client,
        input: {
          view: VIEW,
          query: DEFAULT_QUERY,
          events,
          selectedId,
          now: new Date(),
          onArrival: (event, animate) => {
            arrivals(event, animate);
          },
        },
        reload: {
          schedule: (after, refetch) => {
            schedule(after, refetch);
          },
          cancel: () => undefined,
        },
        inBurst: () => false,
        arrivalBurst: () => false,
        isReady: () => stream.ready,
        refetch: async () => {
          await Promise.resolve();
        },
      },
    );
  };
  return { client, schedule, arrivals, stream, apply };
}

describe(applyUpdate, () => {
  it("patches the view's entry in place, keeping the load's time, and marks other views stale", () => {
    expect.hasAssertions();
    const { client, schedule, apply } = harness();
    const here = eventsKey(VIEW, DEFAULT_QUERY);
    const there = eventsKey(ELSEWHERE, DEFAULT_QUERY);
    client.setQueryData(here, list([recent("a")]), { updatedAt: LOADED_AT });
    client.setQueryData(there, list([]));

    apply({ kind: "created", event: recent("b") });

    expect(client.getQueryData<EventList>(here)?.events.map((each) => each.id)).toStrictEqual(["a", "b"]);
    expect(client.getQueryState(here)?.dataUpdatedAt).toBe(LOADED_AT);
    expect([client.getQueryState(here)?.isInvalidated, client.getQueryState(there)?.isInvalidated]).toStrictEqual([
      false,
      true,
    ]);
    expect(schedule).not.toHaveBeenCalled();
  });

  it("shows an open event its source removed as gone at once, under either detail key", () => {
    expect.hasAssertions();
    const { client, apply } = harness();
    client.setQueryData(eventsKey(VIEW, DEFAULT_QUERY), list([recent("a")]));
    client.setQueryData(["event", "a", "2026-11-26T11:00:00Z"], { found: true, detail: {} });

    apply({ kind: "removed", id: "a" }, "a");

    expect(client.getQueryData(["event", "a", "2026-11-26T11:00:00Z"])).toStrictEqual({ found: false });
    expect(client.getQueryData(["event", "a", undefined])).toStrictEqual({ found: false });
  });

  it("reloads after a change with nothing loaded (the load on its way may predate it), and at once on ready", () => {
    expect.hasAssertions();
    const { client, schedule, arrivals, apply } = harness();

    apply({ kind: "created", event: recent("b") });

    expect(client.getQueryData(eventsKey(VIEW, DEFAULT_QUERY))).toBeUndefined();
    expect(schedule).toHaveBeenLastCalledWith(500, expect.any(Function));
    // Before the stream's `ready`, a new event is backlog: no arrival.
    expect(arrivals).not.toHaveBeenCalled();

    apply({ kind: "ready" });

    expect(schedule).toHaveBeenLastCalledWith(0, expect.any(Function));
  });

  it("announces a live arrival in view and matching; one already drawn is toasted, not animated", () => {
    expect.hasAssertions();
    const { client, arrivals, stream, apply } = harness();
    client.setQueryData(eventsKey(VIEW, DEFAULT_QUERY), list([recent("a")]));
    stream.ready = true;
    const fresh = recent("b");
    const drawn = recent("a");

    apply({ kind: "created", event: fresh });
    apply({ kind: "created", event: drawn });

    expect(arrivals.mock.calls).toStrictEqual([
      [fresh, true],
      [drawn, false],
    ]);
  });

  it("doesn't announce an arrival out of view, or one the filters leave out", () => {
    expect.hasAssertions();
    const { arrivals, stream, apply } = harness();
    stream.ready = true;
    const at = new Date().toISOString();

    apply({ kind: "created", event: makeEvent({ id: "far", centroid: { latitude: -33, longitude: 151 } }) });
    apply({
      kind: "created",
      event: makeEvent({
        id: "detection",
        kind: "detection",
        centroid: { latitude: 48, longitude: 2 },
        observedAt: at,
        updatedAt: at,
      }),
    });

    expect(arrivals).not.toHaveBeenCalled();
  });
});

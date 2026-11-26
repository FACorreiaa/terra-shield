import type { EventList } from "@shield/api-client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { makeEvent } from "@/domain/testEvents.ts";
import type { View } from "@/domain/viewport.ts";

import { useConflicts } from "./useConflicts.ts";

// Before any import runs: the API client keeps the `fetch` it was made with.
const get = vi.hoisted(() => {
  const stub = vi.fn<typeof fetch>();
  globalThis.fetch = stub;
  return stub;
});

const SUDAN: View = { zoom: 5, bbox: "20,5,40,25" };
const CHAD: View = { zoom: 5, bbox: "10,5,25,25" };

function answer(ids: readonly string[]): Response {
  const body: EventList = {
    events: ids.map((id) => makeEvent({ id, type: "conflict", severity: id === "big" ? "extreme" : "minor" })),
    clusters: [],
    truncated: false,
    generatedAt: "2026-11-26T12:00:00Z",
  };
  return Response.json(body);
}

/** The query the n-th request sent. */
function sent(call: number): Record<string, string> {
  const [input] = get.mock.calls[call] ?? [];
  const url = input instanceof Request ? input.url : String(input);
  return Object.fromEntries(new URL(url, "http://localhost").searchParams);
}

interface Props {
  view: View;
  shown: boolean;
}

function useHook({ view, shown }: Props): ReturnType<typeof useConflicts> {
  return useConflicts(view, shown);
}

function setup(
  initialProps: Props,
): ReturnType<typeof renderHook<ReturnType<typeof useConflicts>, Props>> & { client: QueryClient } {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }): ReactElement {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return { ...renderHook(useHook, { initialProps, wrapper }), client };
}

describe(useConflicts, () => {
  beforeEach(() => {
    get.mockReset();
  });
  afterEach(cleanup);

  it("asks for nothing and shows nothing while off", () => {
    const { result } = setup({ view: SUDAN, shown: false });
    expect(get).not.toHaveBeenCalled();
    expect(result.current.list).toBeUndefined();
    expect(result.current.events).toStrictEqual([]);
  });

  it("asks for conflicts only, observed, with no window, under its own cache key", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["small", "big"]));
    const { result, client } = setup({ view: SUDAN, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    expect(sent(0)).toStrictEqual({
      type: "conflict",
      kind: "observed",
      status: "active",
      zoom: "5",
      bbox: SUDAN.bbox,
    });
    // Most severe first, as the list shows them.
    expect(result.current.events.map((event) => event.id)).toStrictEqual(["big", "small"]);
    // Never an `events` entry: the live stream patches and reloads those.
    expect(client.getQueryCache().findAll({ queryKey: ["events"] })).toHaveLength(0);
  });

  it("keeps the last answer when a new view fails, and says so", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["a"])).mockResolvedValueOnce(Response.json({ error: "x" }, { status: 503 }));
    const { result, rerender } = setup({ view: SUDAN, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    rerender({ view: CHAD, shown: true });
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    expect(result.current.events.map((event) => event.id)).toStrictEqual(["a"]);
  });

  it("turned off and on again, never shows the old answer, even when the new load fails", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["a"])).mockResolvedValueOnce(Response.json({ error: "x" }, { status: 503 }));
    const { result, rerender } = setup({ view: SUDAN, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    rerender({ view: SUDAN, shown: false });
    rerender({ view: SUDAN, shown: true });
    expect(result.current.list).toBeUndefined();
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    expect(result.current.list).toBeUndefined();
  });

  it("drops everything at once when turned off", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["a"]));
    const { result, rerender } = setup({ view: SUDAN, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    rerender({ view: SUDAN, shown: false });
    expect(result.current.list).toBeUndefined();
    expect(result.current.events).toStrictEqual([]);
    expect(result.current.clusters).toStrictEqual([]);
  });
});

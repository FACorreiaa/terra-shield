import type { components } from "@shield/api-client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import type { View } from "@/domain/viewport.ts";

import { useDetections } from "./useDetections.ts";

type FireCollection = components["schemas"]["FireFeatureCollection"];

// Before any import runs: the API client keeps the `fetch` it was made with.
const get = vi.hoisted(() => {
  const stub = vi.fn<typeof fetch>();
  globalThis.fetch = stub;
  return stub;
});

const AFRICA: View = { zoom: 3, bbox: "-20,-35,55,38" };
const ASIA: View = { zoom: 3, bbox: "60,0,140,50" };
const NINE = "2026-11-26T09:00:00Z";
const ELEVEN = "2026-11-26T11:00:00Z";

function respond(status: number, body: unknown): Response {
  return Response.json(body, { status, headers: { "Content-Type": "application/geo+json" } });
}

function grid(latestAt: string): Response {
  const body: FireCollection = {
    type: "FeatureCollection",
    aggregation: "grid",
    cellDegrees: 1.40625,
    truncated: false,
    features: [
      { type: "Feature", geometry: { type: "Point", coordinates: [20, 0] }, properties: { count: 3, latestAt } },
    ],
  };
  return respond(200, body);
}

/** A fresh answer per request: a Response's body reads once. */
function answerNine(): Promise<Response> {
  return Promise.resolve(grid(NINE));
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

function useHook({ view, shown }: Props): ReturnType<typeof useDetections> {
  return useDetections(view, shown);
}

function setup(initialProps: Props): ReturnType<typeof renderHook<ReturnType<typeof useDetections>, Props>> {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }): ReactElement {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return renderHook(useHook, { initialProps, wrapper });
}

describe(useDetections, () => {
  beforeEach(() => {
    get.mockReset();
  });
  afterEach(cleanup);

  it("asks for nothing and draws nothing while off", () => {
    const { result } = setup({ view: AFRICA, shown: false });
    expect(get).not.toHaveBeenCalled();
    expect(result.current.layer.latest).toBeUndefined();
    expect(result.current.loading).toBe(false);
  });

  it("loads the view with its zoom and bbox, and no since", async () => {
    expect.hasAssertions();
    get.mockImplementation(answerNine);
    const { result } = setup({ view: AFRICA, shown: true });
    await waitFor(() => {
      expect(result.current.layer.latest).toStrictEqual(new Date(NINE));
    });
    expect(sent(0)).toStrictEqual({ zoom: "3", bbox: AFRICA.bbox });
  });

  it("keeps the last layer when a new view fails, and says so", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(grid(NINE)).mockResolvedValueOnce(respond(503, { error: "unavailable" }));
    const { result, rerender } = setup({ view: AFRICA, shown: true });
    await waitFor(() => {
      expect(result.current.layer.latest).toBeDefined();
    });
    rerender({ view: ASIA, shown: true });
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    expect(result.current.layer.latest).toStrictEqual(new Date(NINE));
  });

  it("starts afresh when turned on again: no layer from before it was turned off", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(grid(NINE));
    const { result, rerender } = setup({ view: AFRICA, shown: true });
    await waitFor(() => {
      expect(result.current.layer.latest).toBeDefined();
    });
    rerender({ view: AFRICA, shown: false });
    expect(result.current.layer.latest).toBeUndefined();

    get.mockResolvedValueOnce(grid(ELEVEN));
    rerender({ view: ASIA, shown: true });
    // Loading the new view: the old one's layer and its age don't come back meanwhile.
    expect(result.current.layer.latest).toBeUndefined();
    expect(result.current.loading).toBe(true);
    await waitFor(() => {
      expect(result.current.layer.latest).toStrictEqual(new Date(ELEVEN));
    });
  });

  it("keeps the same layer when a reload draws the same", async () => {
    expect.hasAssertions();
    get.mockImplementation(answerNine);
    const { result } = setup({ view: AFRICA, shown: true });
    await waitFor(() => {
      expect(result.current.layer.latest).toBeDefined();
    });
    const first = result.current.layer;
    act(() => {
      result.current.refetch();
    });
    await waitFor(() => {
      expect(get).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.failed).toBe(false);
    expect(result.current.layer).toBe(first);
  });
});

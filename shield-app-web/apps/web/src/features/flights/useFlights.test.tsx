import type { AircraftList } from "@shield/api-client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import type { View } from "@/domain/viewport.ts";

import { useFlights } from "./useFlights.ts";

// Before any import runs: the API client keeps the `fetch` it was made with.
const get = vi.hoisted(() => {
  const stub = vi.fn<typeof fetch>();
  globalThis.fetch = stub;
  return stub;
});

/** Lisbon, within one 5° cell. */
const LISBON: View = { zoom: 8, bbox: "-9.5,38.5,-8.5,39" };
/** Madrid, another cell. */
const MADRID: View = { zoom: 8, bbox: "-4,40,-3,40.5" };
/** Porto, a third view. */
const PORTO: View = { zoom: 8, bbox: "-9,41,-8,41.5" };
/** Europe: far more than four cells, so never asked. */
const EUROPE: View = { zoom: 4, bbox: "-10,35,30,60" };

function answer(hexes: readonly string[]): Response {
  const body: AircraftList = {
    aircraft: hexes.map((hex) => ({
      hex,
      lat: 38.7,
      lon: -9.1,
      onGround: false,
      military: false,
      positionAt: "2026-11-26T12:00:00Z",
    })),
    dataAt: "2026-11-26T12:00:00Z",
    complete: true,
    attribution: "© adsb.lol contributors",
    license: "ODbL 1.0",
    licenseUrl: "https://opendatacommons.org/licenses/odbl/1-0/",
  };
  return Response.json(body);
}

const MINUTE_MS = 60_000;

/** A request that never answers: the view stays loading. */
// oxlint-disable-next-line promise/avoid-new -- nothing else makes a promise that never settles
const PENDING = new Promise<Response>(() => {
  // Never settles.
});

const failure = (status: number): Response => Response.json({ error: "x" }, { status });

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

function useHook({ view, shown }: Props): ReturnType<typeof useFlights> {
  return useFlights(view, shown);
}

function setup(initialProps: Props): ReturnType<typeof renderHook<ReturnType<typeof useFlights>, Props>> {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }): ReactElement {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return renderHook(useHook, { initialProps, wrapper });
}

const hexes = (list: AircraftList | undefined): string[] | undefined => list?.aircraft.map((aircraft) => aircraft.hex);

describe(useFlights, () => {
  beforeEach(() => {
    get.mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("asks for nothing while off", () => {
    const { result } = setup({ view: LISBON, shown: false });
    expect(get).not.toHaveBeenCalled();
    expect(result.current.list).toBeUndefined();
    expect(result.current.needsZoom).toBe(false);
  });

  it("never asks for a view the server would refuse, and says to zoom in", () => {
    const { result } = setup({ view: EUROPE, shown: true });
    expect(get).not.toHaveBeenCalled();
    expect(result.current.needsZoom).toBe(true);
  });

  it("asks for the view's box only", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["a1", "a2"]));
    const { result } = setup({ view: LISBON, shown: true });
    await waitFor(() => {
      expect(hexes(result.current.list)).toStrictEqual(["a1", "a2"]);
    });
    expect(sent(0)).toStrictEqual({ bbox: LISBON.bbox });
  });

  it("says to zoom in when the server refuses the view", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(failure(400));
    const { result } = setup({ view: LISBON, shown: true });
    await waitFor(() => {
      expect(result.current.needsZoom).toBe(true);
    });
    expect(result.current.failed).toBe(false);
    // A refusal isn't asked again but by the timer.
    expect(get.mock.calls[1]).toBeUndefined();
  });

  it("drops another view's positions when the new view fails", async () => {
    expect.hasAssertions();
    get.mockResolvedValueOnce(answer(["a1"])).mockResolvedValue(failure(503));
    const { result, rerender } = setup({ view: LISBON, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    rerender({ view: MADRID, shown: true });
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    expect(result.current.list).toBeUndefined();
  });

  it("keeps the view's positions when its refresh fails, and says so", async () => {
    expect.hasAssertions();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    get.mockResolvedValueOnce(answer(["a1"])).mockResolvedValue(failure(503));
    const { result } = setup({ view: LISBON, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    // The minute's refresh, and nothing sooner: a failure waits for the next one.
    await vi.advanceTimersByTimeAsync(MINUTE_MS);
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    expect(hexes(result.current.list)).toStrictEqual(["a1"]);
  });

  it("after a failed view, never shows an older view's positions while the next one loads", async () => {
    expect.hasAssertions();
    get
      .mockResolvedValueOnce(answer(["a1"]))
      .mockResolvedValueOnce(failure(503))
      .mockReturnValue(PENDING);
    const { result, rerender } = setup({ view: LISBON, shown: true });
    await waitFor(() => {
      expect(result.current.list).toBeDefined();
    });
    rerender({ view: MADRID, shown: true });
    await waitFor(() => {
      expect(result.current.failed).toBe(true);
    });
    rerender({ view: PORTO, shown: true });
    await waitFor(() => {
      expect(get).toHaveBeenCalledTimes(3);
    });
    expect(result.current.list).toBeUndefined();
  });
});

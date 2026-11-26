import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { makeEvent } from "@/domain/testEvents.ts";

import { useArrivals } from "./useArrivals.ts";

const SEVERE = makeEvent({ id: "severe", severity: "severe" });
const MODERATE = makeEvent({ id: "moderate", severity: "moderate" });

describe(useArrivals, () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("toasts a severe arrival for 6 s, and animates every arrival until the map says it has", () => {
    expect.hasAssertions();

    const { result } = renderHook(() => useArrivals());
    act(() => {
      result.current.note(MODERATE, true);
      result.current.note(SEVERE, true);
    });

    expect([...result.current.arrivals]).toStrictEqual(["moderate", "severe"]);
    expect(result.current.toast?.id).toBe("severe");

    act(() => {
      result.current.started("severe");
      result.current.arrived("moderate");
      vi.advanceTimersByTime(6000);
    });

    // Playing, so never forgotten; the toast's 6 s are up.
    expect([...result.current.arrivals]).toStrictEqual(["severe"]);
    expect(result.current.toast).toBeUndefined();
  });

  it("forgets an arrival the map never drew after 5 s, with no other arrival to prompt it", () => {
    expect.hasAssertions();

    const { result } = renderHook(() => useArrivals());
    act(() => {
      result.current.note(MODERATE, true);
    });

    expect(result.current.toast).toBeUndefined();

    act(() => {
      vi.advanceTimersByTime(5100);
    });

    expect(result.current.arrivals.size).toBe(0);
  });

  it("toasts but doesn't animate an event the map already had", () => {
    expect.hasAssertions();

    const { result } = renderHook(() => useArrivals());
    act(() => {
      result.current.note(SEVERE, false);
    });

    expect([result.current.arrivals.size, result.current.toast?.id]).toStrictEqual([0, "severe"]);
  });

  it("lets a newer toast replace the last, holds it while pointed at, and ends it on dismissal", () => {
    expect.hasAssertions();

    const { result } = renderHook(() => useArrivals());
    act(() => {
      result.current.note(SEVERE, true);
      vi.advanceTimersByTime(4000);
      result.current.note(makeEvent({ id: "extreme", severity: "extreme" }), true);
    });
    act(() => {
      result.current.holdToast(true);
    });
    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    expect(result.current.toast?.id).toBe("extreme");

    act(() => {
      result.current.dismissToast();
    });

    expect(result.current.toast).toBeUndefined();
  });
});

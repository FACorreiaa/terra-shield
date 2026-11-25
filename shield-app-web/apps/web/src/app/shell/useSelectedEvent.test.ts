import { cleanup, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vite-plus/test";

import type { DetailAnswer } from "@/features/detail/detailState.ts";

import { useHeldAnswer } from "./useSelectedEvent.ts";

const GONE: DetailAnswer = { found: false };

interface Props {
  id: string | undefined;
  data: DetailAnswer | undefined;
}

function useHeld({ id, data }: Props): DetailAnswer | undefined {
  return useHeldAnswer(id, data);
}

describe(useHeldAnswer, () => {
  afterEach(cleanup);

  it("keeps the last answer for the id while a newer copy's fetch has none", () => {
    const initialProps: Props = { id: "a", data: GONE };
    const { result, rerender } = renderHook(useHeld, { initialProps });
    expect(result.current).toBe(GONE);
    // A new key for a newer copy: pending, then failed, so the query has no data.
    rerender({ id: "a", data: undefined });
    expect(result.current).toBe(GONE);
  });

  it("never lends one event's answer to another", () => {
    const initialProps: Props = { id: "a", data: GONE };
    const { result, rerender } = renderHook(useHeld, { initialProps });
    rerender({ id: "b", data: undefined });
    expect(result.current).toBeUndefined();
    rerender({ id: undefined, data: undefined });
    expect(result.current).toBeUndefined();
  });
});

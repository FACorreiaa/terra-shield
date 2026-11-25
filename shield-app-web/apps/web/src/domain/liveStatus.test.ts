import { describe, expect, it } from "vite-plus/test";

import type { LoadState } from "./liveStatus.ts";
import { liveStatus } from "./liveStatus.ts";

const NOW = new Date("2026-11-25T14:30:00Z");
const MINUTE_MS = 60_000;
const CLOCK_1402 = new Date("2026-11-25T14:02:00Z");

function minutesAgo(minutes: number): Date {
  return new Date(NOW.getTime() - minutes * MINUTE_MS);
}

function state(extra: Partial<LoadState>): LoadState {
  return { lastSuccess: undefined, loading: false, offline: false, failed: false, now: NOW, ...extra };
}

/** The status with the last success `minutes` ago. */
function after(minutes: number, extra: Partial<LoadState> = {}): ReturnType<typeof liveStatus> {
  return liveStatus(state({ lastSuccess: minutesAgo(minutes), ...extra }));
}

describe(liveStatus, () => {
  it("says it is loading before the first events arrive", () => {
    expect(liveStatus(state({ loading: true }))).toStrictEqual({
      text: "Loading events…",
      dot: "ok",
      warns: false,
      retry: false,
    });
  });

  it("gives the age while fresh, and turns to a warning after three missed refreshes", () => {
    expect(after(2)).toMatchObject({ text: "Updated 2 min ago", dot: "ok" });
    expect(after(3)).toMatchObject({ warns: false });
    expect(after(14)).toMatchObject({
      text: "Last update 14 min ago",
      dot: "warning",
      warns: true,
    });
  });

  it("keeps the data offline and says from when, with a retry", () => {
    expect(liveStatus(state({ offline: true, lastSuccess: CLOCK_1402 }))).toStrictEqual({
      text: "Offline · showing data from 14:02 UTC",
      dot: "muted",
      warns: true,
      retry: true,
    });
    expect(liveStatus(state({ offline: true })).text).toBe("Offline");
  });

  it("marks a failed refresh with a danger dot but keeps the last age", () => {
    expect(after(1, { failed: true })).toMatchObject({
      text: "Updated 1 min ago",
      dot: "danger",
    });
  });
});

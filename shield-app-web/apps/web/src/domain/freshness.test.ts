import { describe, expect, it } from "vite-plus/test";

import { isStale, label, relative, resolvedWords, stamp, utcClock, utcDay } from "./freshness.ts";
import { AT, SAMPLE_NOW, sampleEvent } from "./testEvents.ts";

const now = SAMPLE_NOW;
const ago = (seconds: number, from: Date = now): Date => new Date(from.getTime() - seconds * 1000);

describe(relative, () => {
  it.each([
    [5, "now"],
    [45, "45 s ago"],
    [12 * 60, "12 min ago"],
    [3 * 3600, "3 h ago"],
    [2 * 86_400, "2 d ago"],
    [-600, "in 10 min"],
  ])("reads %i s as %s", (seconds, expected) => {
    expect(relative(ago(seconds), now)).toBe(expected);
  });
});

describe(isStale, () => {
  it("turns stale after three poll intervals", () => {
    expect(isStale(ago(179), 60, now)).toBe(false);
    expect(isStale(ago(181), 60, now)).toBe(true);
  });
});

describe(utcClock, () => {
  it("formats hours and minutes in UTC", () => {
    expect(utcClock(now)).toBe("08:00 UTC");
  });
});

describe(label, () => {
  it("pairs the relative time with the clock", () => {
    expect(label(ago(720, AT), AT)).toBe("12 min ago · 14:08 UTC");
  });
});

describe(utcDay, () => {
  it("names the day and month in UTC, September as Sep", () => {
    expect(utcDay(new Date("2026-09-10T00:00:00Z"))).toBe("10 Sep");
    expect(utcDay(new Date("2026-08-09T23:59:00Z"))).toBe("9 Aug");
  });
});

describe(stamp, () => {
  it.each([
    [720, "12 min ago · 14:08 UTC"],
    [3 * 86_400, "3 d ago · 13 Nov, 14:20 UTC"],
    [-2 * 86_400, "in 2 d · 18 Nov, 14:20 UTC"],
  ])("reads %i s ago as %s", (seconds, expected) => {
    expect(stamp(ago(seconds, AT), AT)).toBe(expected);
  });
});

describe(resolvedWords, () => {
  const resolved = sampleEvent(-1);

  it("prefers an end time that has passed", () => {
    expect(resolvedWords(resolved, now)).toBe("Resolved · ended 50 min ago");
  });

  it("falls back to the last update without an end or with one still ahead", () => {
    const { endedAt: _ended, ...noEnd } = resolved;
    expect(resolvedWords(noEnd, now)).toBe("Resolved · updated 50 min ago");
    const futureEnd = { ...resolved, endedAt: new Date(now.getTime() + 3_600_000).toISOString() };
    expect(resolvedWords(futureEnd, now)).toBe("Resolved · updated 50 min ago");
  });

  it("says nothing for an active event", () => {
    expect(resolvedWords(sampleEvent(0), now)).toBeUndefined();
  });
});

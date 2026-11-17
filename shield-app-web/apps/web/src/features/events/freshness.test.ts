import { describe, expect, it } from "vite-plus/test";

import { isStale, relative, utcClock } from "./freshness.ts";

const now = new Date("2027-01-15T08:00:00Z");
const ago = (seconds: number): Date => new Date(now.getTime() - seconds * 1000);

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

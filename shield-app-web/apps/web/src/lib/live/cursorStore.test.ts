import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { cursorStore, isAfter } from "./cursorStore.ts";

describe(isAfter, () => {
  it("compares decimal cursors by value, whatever their length", () => {
    expect([isAfter("10", "9"), isAfter("9", "10"), isAfter("42", "42"), isAfter("1", undefined)]).toStrictEqual([
      true,
      false,
      false,
      true,
    ]);
    expect(isAfter(undefined, "1")).toBe(false);
  });
});

describe(cursorStore, () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("never moves the stored cursor back: a deposed leader's late write is ignored", () => {
    expect.hasAssertions();
    const deposed = cursorStore();
    const leader = cursorStore();
    leader.save("50");
    deposed.save("45");

    expect(cursorStore().read()).toBe("50");
  });

  it("writes a burst at most once a second, and reads the newest meanwhile", () => {
    expect.hasAssertions();
    const store = cursorStore();
    store.save("41");
    store.save("42");
    store.save("43");

    expect([store.read(), cursorStore().read()]).toStrictEqual(["43", "41"]);

    vi.advanceTimersByTime(1000);

    expect(cursorStore().read()).toBe("43");
  });

  it("forgets the cursor at once on a drop, so an earlier one can follow (a database reset)", () => {
    expect.hasAssertions();
    const store = cursorStore();
    store.save("900");
    store.drop();

    expect(cursorStore().read()).toBeUndefined();

    vi.advanceTimersByTime(1000);
    store.save("3");

    expect(cursorStore().read()).toBe("3");
  });
});

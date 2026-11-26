import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import type { Following } from "./connect.ts";
import type { CursorStore } from "./cursorStore.ts";
import type { Channel, Locks, Relay } from "./feedNode.ts";
import { FeedNode } from "./feedNode.ts";
import type { LiveUpdate } from "./liveChange.ts";

interface Request {
  onGranted: () => Promise<void>;
  reject: (reason: unknown) => void;
}

/** One lock, granted in order; `steal` takes it, rejecting the holder's request as browsers do. */
function fakeLocks(): { locks: Locks; close: () => void } {
  const state: { holder: Request | undefined; queue: Request[] } = { holder: undefined, queue: [] };
  const grant = (request: Request): void => {
    state.holder = request;
    void request.onGranted();
  };
  const locks: Locks = {
    request: async (_name, options, onGranted) => {
      // oxlint-disable-next-line promise/avoid-new -- a lock request settles from outside
      await new Promise<void>((_resolve, reject) => {
        const request = { onGranted, reject };
        // As in browsers, the signal withdraws a queued request only; a granted one ignores it.
        options.signal?.addEventListener("abort", () => {
          if (state.queue.includes(request)) {
            state.queue = state.queue.filter((each) => each !== request);
            reject(new DOMException("Withdrawn", "AbortError"));
          }
        });
        if (options.steal === true) {
          state.holder?.reject(new DOMException("Stolen", "AbortError"));
          grant(request);
        } else if (state.holder === undefined) {
          grant(request);
        } else {
          state.queue.push(request);
        }
      });
    },
  };
  /** The holder's tab closes: the lock passes on. */
  const close = (): void => {
    const [next, ...rest] = state.queue;
    state.holder = undefined;
    state.queue = rest;
    if (next !== undefined) {
      grant(next);
    }
  };
  return { locks, close };
}

/** A channel per tab on one bus; a frozen tab neither sends nor hears. */
function bus(): { channel: (tab: number) => Channel; frozen: Set<number> } {
  const listeners = new Map<number, (message: MessageEvent<Relay>) => void>();
  const frozen = new Set<number>();
  const channel = (tab: number): Channel => ({
    addEventListener: (_type, listener: (message: MessageEvent<Relay>) => void) => {
      listeners.set(tab, listener);
    },
    postMessage: (relay: Relay) => {
      if (frozen.has(tab)) {
        return;
      }
      for (const [other, listener] of listeners) {
        if (other !== tab && !frozen.has(other)) {
          listener(new MessageEvent("message", { data: structuredClone(relay) }));
        }
      }
    },
  });
  return { channel, frozen };
}

interface FakeStream {
  follow: (following: Following) => Promise<void>;
  cursors: (string | undefined)[];
  say: (update: LiveUpdate) => void;
  open: () => boolean;
}

/** A stream per tab, driven by the test: what it was asked to resume from, and a way to speak. */
function fakeStream(): FakeStream {
  const cursors: (string | undefined)[] = [];
  const state: { following: Following | undefined } = { following: undefined };
  return {
    cursors,
    follow: async (following) => {
      state.following = following;
      cursors.push(following.cursor());
      // oxlint-disable-next-line promise/avoid-new -- runs until the signal aborts
      await new Promise<void>((resolve) => {
        following.signal.addEventListener("abort", () => {
          resolve();
        });
      });
    },
    say: (update) => {
      state.following?.onUpdate(update);
    },
    open: () => state.following !== undefined && !state.following.signal.aborted,
  };
}

function memoryCursor(): CursorStore {
  let cursor: string | undefined = "40";
  return {
    read: () => cursor,
    save: (next) => {
      cursor = next;
    },
    drop: () => {
      cursor = undefined;
    },
  };
}

interface Tab {
  node: FeedNode;
  stream: FakeStream;
  heard: LiveUpdate[];
}

interface Shared {
  locks: ReturnType<typeof fakeLocks> | undefined;
  bus: ReturnType<typeof bus>;
  cursor: CursorStore;
}

function openTab(id: number, shared: Shared, visible: () => boolean = () => true): Tab {
  const stream = fakeStream();
  const node = new FeedNode({
    locks: shared.locks?.locks,
    channel: shared.locks === undefined ? undefined : shared.bus.channel(id),
    follow: stream.follow,
    cursor: shared.cursor,
    now: () => Date.now(),
    visible,
  });
  const heard: LiveUpdate[] = [];
  node.onUpdate((update) => {
    heard.push(update);
  });
  node.start();
  return { node, stream, heard };
}

const READY: LiveUpdate = { change: { kind: "ready" }, cursor: "41" };
const HEARTBEAT: LiveUpdate = { change: undefined, cursor: "42" };

describe(FeedNode, () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("streams for itself without Web Locks", async () => {
    expect.hasAssertions();
    const one = openTab(1, { locks: undefined, bus: bus(), cursor: memoryCursor() });
    await vi.advanceTimersByTimeAsync(0);
    expect(one.stream.cursors).toStrictEqual(["40"]);
    one.stream.say(READY);
    expect(one.heard).toStrictEqual([READY]);
    expect(one.node.status.ready).toBe(true);
  });

  it("opens one stream for many tabs and relays it, the ready state too for a late tab", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const first = openTab(1, shared);
    const second = openTab(2, shared);
    await vi.advanceTimersByTimeAsync(0);
    first.stream.say(READY);
    first.stream.say(HEARTBEAT);
    expect(second.heard).toStrictEqual([READY, HEARTBEAT]);
    expect(second.node.status.ready).toBe(true);
    const late = openTab(3, shared);
    await vi.advanceTimersByTimeAsync(0);
    expect(late.node.status.ready).toBe(true);
    expect([first, second, late].map((each) => each.stream.cursors.length)).toStrictEqual([1, 0, 0]);
  });

  it("hands the stream on, from the saved cursor, when the leader closes", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const first = openTab(1, shared);
    const second = openTab(2, shared);
    await vi.advanceTimersByTimeAsync(0);
    first.stream.say(READY);
    first.stream.say(HEARTBEAT);
    shared.bus.frozen.add(1);
    shared.locks.close();
    await vi.advanceTimersByTimeAsync(0);
    expect(second.stream.cursors).toStrictEqual(["42"]);
    expect(second.node.leading).toBe(true);
  });

  it("takes the stream from a frozen leader, which stops its own", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const first = openTab(1, shared);
    const second = openTab(2, shared);
    await vi.advanceTimersByTimeAsync(0);
    // The leader's pulse keeps a quiet stream's followers patient.
    await vi.advanceTimersByTimeAsync(300_000);
    expect(second.node.leading).toBe(false);
    shared.bus.frozen.add(1);
    await vi.advanceTimersByTimeAsync(210_000);
    expect(second.node.leading).toBe(true);
    expect(second.stream.open()).toBe(true);
    expect(first.stream.open()).toBe(false);
    expect(first.node.leading).toBe(false);
  });

  it("asks after a leader it hasn't heard from, and a well one keeps the stream", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const awake = { now: true };
    const first = openTab(1, shared);
    const second = openTab(2, shared, () => awake.now);
    await vi.advanceTimersByTimeAsync(0);
    // The follower sleeps (hearing nothing, checking nothing), then wakes with the leader long unheard.
    shared.bus.frozen.add(2);
    awake.now = false;
    await vi.advanceTimersByTimeAsync(300_000);
    shared.bus.frozen.delete(2);
    awake.now = true;
    await vi.advanceTimersByTimeAsync(60_000);
    expect(second.node.leading).toBe(false);
    expect(first.stream.open()).toBe(true);
    expect(first.stream.cursors).toHaveLength(1);
  });

  it("is not live after a takeover until the new leader's own ready, in every tab", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const [first, second, third] = [openTab(1, shared), openTab(2, shared), openTab(3, shared)];
    await vi.advanceTimersByTimeAsync(0);
    first.stream.say(READY);
    expect(third.node.status.ready).toBe(true);
    shared.bus.frozen.add(1);
    shared.locks.close();
    await vi.advanceTimersByTimeAsync(0);
    expect([second.node.status.ready, third.node.status.ready]).toStrictEqual([false, false]);
    second.stream.say({ change: { kind: "ready" }, cursor: "43" });
    expect([second.node.status.ready, third.node.status.ready]).toStrictEqual([true, true]);
  });

  it("drops a relayed update older than one already heard, until a new run starts", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const first = openTab(1, shared);
    const second = openTab(2, shared);
    await vi.advanceTimersByTimeAsync(0);
    const late: LiveUpdate = { change: undefined, cursor: "45" };
    const reset: LiveUpdate = { change: { kind: "reset" }, cursor: "3" };
    first.stream.say({ change: undefined, cursor: "50" });
    first.stream.say(late);
    first.stream.say(reset);
    expect(second.heard.map((each) => each.cursor)).toStrictEqual(["50", "3"]);
  });

  it("tells the page of activity at most once a second, and of ready at once", async () => {
    expect.hasAssertions();
    const shared = { locks: fakeLocks(), bus: bus(), cursor: memoryCursor() };
    const first = openTab(1, shared);
    const second = openTab(2, shared);
    await vi.advanceTimersByTimeAsync(0);
    let told = 0;
    second.node.onStatus(() => {
      told += 1;
    });
    first.stream.say(READY);
    for (const cursor of ["42", "43", "44", "45"]) {
      first.stream.say({ change: undefined, cursor });
    }
    expect(told).toBe(1);
    await vi.advanceTimersByTimeAsync(1000);
    first.stream.say({ change: undefined, cursor: "46" });
    expect(told).toBe(2);
  });
});

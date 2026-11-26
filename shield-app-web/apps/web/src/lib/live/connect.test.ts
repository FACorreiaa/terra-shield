import { describe, expect, it } from "vite-plus/test";

import type { Connection, Following } from "./connect.ts";
import { connect, follow, streamBackoff, StreamRefusedError } from "./connect.ts";
import type { LiveUpdate } from "./liveChange.ts";

function body(text: string): ReadableStream<Uint8Array> {
  const bytes = new TextEncoder().encode(text);
  return new ReadableStream({
    start(controller) {
      // Split mid-character ("É" is two bytes), as the network may.
      controller.enqueue(bytes.slice(0, 20));
      controller.enqueue(bytes.slice(20));
      controller.close();
    },
  });
}

interface Seen {
  headers: Headers[];
}

function fetcher(seen: Seen, answers: readonly Response[]): typeof fetch {
  let next = 0;
  return (_input, init) => {
    seen.headers.push(new Headers(init?.headers));
    const answer = answers[next];
    next += 1;
    return Promise.resolve(answer ?? Response.error());
  };
}

describe(connect, () => {
  it("sends the cursor and reads changes and heartbeats", async () => {
    expect.hasAssertions();
    const seen: Seen = { headers: [] };
    const updates: LiveUpdate[] = [];
    const onUpdate = (update: LiveUpdate): void => {
      updates.push(update);
    };
    const text =
      'retry: 5000\n\nid: 8\nevent: ready\ndata: {}\n\nid: 9\n\nid: 10\nevent: event.removed\ndata: {"id":"Évora"}\n\n';
    const answer = new Response(body(text));
    await connect({ cursor: "7", signal: new AbortController().signal, onUpdate, fetcher: fetcher(seen, [answer]) });
    expect(seen.headers[0]?.get("Last-Event-ID")).toBe("7");
    expect(updates).toStrictEqual([
      { change: { kind: "ready" }, cursor: "8" },
      { change: undefined, cursor: "9" },
      { change: { kind: "removed", id: "Évora" }, cursor: "10" },
    ]);
  });

  it("reports a refusal with the server's Retry-After", async () => {
    expect.hasAssertions();
    const seen: Seen = { headers: [] };
    const refused = new Response("{}", { status: 503, headers: { "Retry-After": "30" } });
    const attempt = connect({
      cursor: undefined,
      signal: new AbortController().signal,
      onUpdate: () => undefined,
      fetcher: fetcher(seen, [refused]),
    });
    await expect(attempt).rejects.toStrictEqual(new StreamRefusedError(503, 30_000));
    expect(seen.headers[0]?.has("Last-Event-ID")).toBe(false);
  });
});

interface Scripted {
  following: Following;
  cursors: (string | undefined)[];
  slept: number[];
  downs: () => number;
}

/** A follow run whose connections play `script` in turn, then stop it; sleeps are recorded, not slept. */
function scripted(script: readonly ((connection: Connection) => void)[]): Scripted {
  const controller = new AbortController();
  const state: { cursor: string | undefined; downs: number } = { cursor: undefined, downs: 0 };
  const cursors: (string | undefined)[] = [];
  const slept: number[] = [];
  const following: Following = {
    cursor: () => state.cursor,
    onUpdate: (update) => {
      state.cursor = update.cursor;
    },
    onDown: () => {
      state.downs += 1;
    },
    dropCursor: () => {
      state.cursor = undefined;
    },
    signal: controller.signal,
    connector: (connection) => {
      cursors.push(connection.cursor);
      const step = script[cursors.length - 1];
      if (step === undefined) {
        controller.abort();
        return Promise.resolve();
      }
      try {
        step(connection);
        return Promise.resolve();
      } catch (error) {
        return Promise.reject(error instanceof Error ? error : new Error("failed"));
      }
    },
    sleep: (ms) => {
      slept.push(ms);
      return Promise.resolve();
    },
    jitter: () => 1,
  };
  return { following, cursors, slept, downs: () => state.downs };
}

describe(follow, () => {
  it("backs off doubling to a minute", () => {
    expect([1, 2, 3, 4, 5, 6].map((failures) => streamBackoff(failures))).toStrictEqual([
      5000, 10_000, 20_000, 40_000, 60_000, 60_000,
    ]);
  });

  it("reconnects with the last cursor, honours Retry-After, and a heartbeat resets the count", async () => {
    expect.hasAssertions();
    const run = scripted([
      () => {
        throw new Error("network");
      },
      () => {
        throw new StreamRefusedError(429, 7000);
      },
      ({ onUpdate }) => {
        onUpdate({ change: undefined, cursor: "5" });
      },
      () => {
        throw new StreamRefusedError(400);
      },
    ]);
    await follow(run.following);
    expect(run.cursors).toStrictEqual([undefined, undefined, undefined, "5", undefined]);
    // The heartbeat reset the failures: 5 s again after it, not 20 s.
    expect(run.slept).toStrictEqual([5000, 7000, 5000, 10_000]);
    // Each connection that ended went down, but not the last: stopped on purpose, nothing is down.
    expect(run.downs()).toBe(4);
  });
});

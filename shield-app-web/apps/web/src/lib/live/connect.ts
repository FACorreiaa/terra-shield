import type { LiveUpdate } from "./liveChange.ts";
import { liveChange } from "./liveChange.ts";
import type { SSEOutput } from "./sse.ts";
import { SSEStreamDecoder } from "./sse.ts";

const STREAM_URL = "/v1/stream";
const MS_PER_SECOND = 1000;
/** Two missed heartbeats (every 20 s), plus slack: a silent connection is a dead one (a half-open socket after sleep). */
const SILENCE_MS = 45_000;
/** The server's `retry:` hint. */
const FIRST_BACKOFF_MS = 5000;
const MAX_BACKOFF_MS = 60_000;
/** ±20 %, so clients dropped together (a pod shutting down) don't come back together. */
const JITTER = 0.2;
const HTTP = { ok: 200, badRequest: 400, tooMany: 429, unavailable: 503 } as const;

/**
 * The stream refused the connection: for now (429, 503; reconnect after `retryAfterMs` when given),
 * or for good with this cursor (400).
 */
class StreamRefusedError extends Error {
  public override readonly name = "StreamRefusedError";
  public readonly status: number;
  public readonly retryAfterMs: number | undefined;

  public constructor(status: number, retryAfterMs?: number) {
    super(`The stream answered ${status}`);
    this.status = status;
    this.retryAfterMs = retryAfterMs;
  }
}

interface Connection {
  cursor: string | undefined;
  signal: AbortSignal;
  onUpdate: (update: LiveUpdate) => void;
  /** `fetch`, replaceable in tests. */
  fetcher?: typeof fetch;
}

function retryAfter(response: Response): number | undefined {
  const value = response.headers.get("Retry-After");
  return value !== null && /^\d+$/u.test(value) ? Number(value) * MS_PER_SECOND : undefined;
}

function checked(response: Response): ReadableStream<Uint8Array> {
  if (response.status === HTTP.tooMany || response.status === HTTP.unavailable) {
    throw new StreamRefusedError(response.status, retryAfter(response));
  }
  if (response.status !== HTTP.ok || response.body === null) {
    throw new StreamRefusedError(response.status);
  }
  return response.body;
}

function update(output: SSEOutput): LiveUpdate {
  return output.kind === "cursor"
    ? { change: undefined, cursor: output.id }
    : { change: liveChange(output.message), cursor: output.message.id };
}

/** Ends the connection when nothing, not even a heartbeat, has come for `SILENCE_MS`. */
function watchdog(controller: AbortController): { poke: () => void; stop: () => void } {
  const silent = (): void => {
    controller.abort(new Error("The stream went silent"));
  };
  let timer = setTimeout(silent, SILENCE_MS);
  return {
    poke: () => {
      clearTimeout(timer);
      timer = setTimeout(silent, SILENCE_MS);
    },
    stop: () => {
      clearTimeout(timer);
    },
  };
}

interface Reading {
  cursor: string | undefined;
  own: AbortController;
  onUpdate: (update: LiveUpdate) => void;
  poke: () => void;
}

async function read(body: ReadableStream<Uint8Array>, { cursor, own, onUpdate, poke }: Reading): Promise<void> {
  const reader = body.getReader();
  const text = new TextDecoder();
  const decoder = new SSEStreamDecoder(cursor);
  const cancel = async (): Promise<void> => {
    try {
      await reader.cancel();
    } catch {
      // Already closed.
    }
  };
  own.signal.addEventListener(
    "abort",
    () => {
      void cancel();
    },
    { once: true },
  );
  for (;;) {
    // oxlint-disable-next-line no-await-in-loop -- a stream is read one chunk after the other
    const { done, value } = await reader.read();
    own.signal.throwIfAborted();
    if (done) {
      return;
    }
    poke();
    for (const output of decoder.feed(text.decode(value, { stream: true }))) {
      onUpdate(update(output));
    }
  }
}

/**
 * One connection to `/v1/stream`, unfiltered (LiveStream.swift): the map filters by view itself
 * rather than reconnecting on every pan. Resolves when the server ends it; rejects on a refusal, a
 * network failure, silence, or the signal.
 */
async function connect({ cursor, signal, onUpdate, fetcher = fetch }: Connection): Promise<void> {
  const own = new AbortController();
  const stopOwn = (): void => {
    own.abort(signal.reason);
  };
  signal.addEventListener("abort", stopOwn, { once: true });
  const silence = watchdog(own);
  try {
    const headers: Record<string, string> = { Accept: "text/event-stream", "Cache-Control": "no-cache" };
    const response = await fetcher(STREAM_URL, {
      headers: cursor === undefined ? headers : { ...headers, "Last-Event-ID": cursor },
      signal: own.signal,
      cache: "no-store",
    });
    await read(checked(response), { cursor, own, onUpdate, poke: silence.poke });
  } finally {
    silence.stop();
    signal.removeEventListener("abort", stopOwn);
  }
}

/** 5 s, 10 s, 20 s, 40 s, then 60 s. */
function streamBackoff(failures: number): number {
  return Math.min(FIRST_BACKOFF_MS * 2 ** (Math.max(failures, 1) - 1), MAX_BACKOFF_MS);
}

function jittered(): number {
  return 1 - JITTER + Math.random() * 2 * JITTER;
}

/** Waits `ms`, or less when `signal` aborts first. */
async function pause(ms: number, signal: AbortSignal): Promise<void> {
  // oxlint-disable-next-line promise/avoid-new -- a timer is a callback API
  await new Promise<void>((resolve) => {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      // The signal lives as long as the leadership: each pause takes its listener with it.
      controller.abort();
      resolve();
    }, ms);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(timer);
        resolve();
      },
      { once: true, signal: controller.signal },
    );
  });
}

interface Following {
  /** The cursor to resume from, read at each connection. */
  cursor: () => string | undefined;
  onUpdate: (update: LiveUpdate) => void;
  /** The connection ended: not live until the next `ready`. */
  onDown: () => void;
  /** The server refused the cursor (400): forget it. */
  dropCursor: () => void;
  signal: AbortSignal;
  connector?: (connection: Connection) => Promise<void>;
  sleep?: (ms: number, signal: AbortSignal) => Promise<void>;
  jitter?: () => number;
}

/** How long to wait after a connection ended with `error`; drops a refused cursor on the way. */
function delayAfter(error: unknown, failures: number, options: Following): number {
  if (error instanceof StreamRefusedError && error.status === HTTP.badRequest) {
    options.dropCursor();
  }
  // The server's Retry-After is taken as given.
  if (error instanceof StreamRefusedError && error.retryAfterMs !== undefined) {
    return error.retryAfterMs;
  }
  return streamBackoff(failures) * (options.jitter ?? jittered)();
}

/** How a connection ended: undefined when the server closed it, else why. */
async function outcome(connection: Promise<void>): Promise<unknown> {
  try {
    await connection;
    return undefined;
  } catch (error) {
    return error;
  }
}

/**
 * Follows `/v1/stream` until `signal` aborts (LiveMapModel `followChanges`), reconnecting with the
 * last cursor. Never a banner: the 60 s poll carries the map meanwhile.
 */
async function follow(options: Following): Promise<void> {
  const { signal, connector = connect, sleep = pause } = options;
  let failures = 0;
  const onUpdate = (next: LiveUpdate): void => {
    failures = 0;
    options.onUpdate(next);
  };
  while (!signal.aborted) {
    // oxlint-disable-next-line no-await-in-loop -- one connection at a time, by design
    const ended = await outcome(connector({ cursor: options.cursor(), signal, onUpdate }));
    // Stopped on purpose (this tab no longer leads): the stream isn't down, another tab has it.
    if (signal.aborted) {
      return;
    }
    options.onDown();
    failures += 1;
    // oxlint-disable-next-line no-await-in-loop -- the backoff between connections
    await sleep(delayAfter(ended, failures, options), signal);
  }
}

export type { Connection, Following };
export { connect, follow, streamBackoff, StreamRefusedError };

import type { Following } from "./connect.ts";
import type { CursorStore } from "./cursorStore.ts";
import { isAfter } from "./cursorStore.ts";
import type { LiveUpdate } from "./liveChange.ts";

/** Whether the stream is caught up (`ready` came and no drop since), and when it last said anything. */
interface FeedStatus {
  ready: boolean;
  /** Milliseconds since the epoch; undefined before the stream's first word. */
  lastActivity: number | undefined;
}

/** What tabs say to each other on the channel. */
type Relay =
  | { kind: "update"; update: LiveUpdate }
  | { kind: "down" }
  /** A tab that just opened asks for the state; the leader answers. */
  | { kind: "hello" }
  | ({ kind: "state" } & FeedStatus)
  /** The leader's pulse, while its stream is down too. */
  | { kind: "alive" };

/** The part of Web Locks the feed uses. */
interface Locks {
  request: (name: string, options: LockOptions, callback: () => Promise<void>) => Promise<void>;
}

/** The part of BroadcastChannel the feed uses. */
interface Channel {
  postMessage: (relay: Relay) => void;
  addEventListener: (type: "message", listener: (message: MessageEvent<Relay>) => void) => void;
}

/** The parts of the browser the feed uses, replaceable in tests. */
interface FeedEnv {
  /** Undefined without Web Locks or BroadcastChannel: every tab streams for itself. */
  locks: Locks | undefined;
  channel: Channel | undefined;
  follow: (following: Following) => Promise<void>;
  cursor: CursorStore;
  now: () => number;
  visible: () => boolean;
}

const LOCK = "shield-stream";
const PULSE_MS = 20_000;
const CHECK_MS = 15_000;
/**
 * A leader unheard for this long may be frozen (a background tab the browser stopped), or only
 * throttled (a hidden tab's timers fire once a minute), or the follower itself slept: it is asked.
 */
const SILENT_MS = 45_000;
/**
 * A leader that doesn't answer a `hello` within this is frozen, and another tab takes the stream.
 * Messages aren't throttled like timers, so a hidden but running leader answers at once.
 */
const PROBE_MS = 5000;
/** Followers re-render at most this often for activity alone; a `ready` flip shows at once. */
const PUBLISH_EVERY_MS = 1000;
const NOT_STARTED: FeedStatus = { ready: false, lastActivity: undefined };

/**
 * One tab's part in sharing one `/v1/stream` per browser (the server allows 8 per IP). The tab
 * holding the `shield-stream` lock streams and relays everything over the `shield-live` channel;
 * the others listen. When the leader closes, the lock passes to the next tab, which resumes from the
 * stored cursor.
 */
class FeedNode {
  /** The freshest word: what a `hello` is answered with. */
  #status = NOT_STARTED;
  /** What the page sees: `#status`, at most once a second for activity alone. */
  #published = NOT_STARTED;
  readonly #updateListeners = new Set<(update: LiveUpdate) => void>();
  readonly #statusListeners = new Set<() => void>();
  readonly #env: FeedEnv;
  #leading = false;
  #run: AbortController | undefined = undefined;
  #queued: AbortController | undefined = undefined;
  #lastHeard: number;
  #probing = false;
  /** The newest relayed cursor: a deposed leader's late relays fall behind it and are dropped. */
  #highest: string | undefined = undefined;

  public constructor(env: FeedEnv) {
    this.#env = env;
    this.#lastHeard = env.now();
  }

  public start(): void {
    const { locks, channel } = this.#env;
    if (locks === undefined || channel === undefined) {
      void this.#lead();
      return;
    }
    channel.addEventListener("message", (message: MessageEvent<Relay>) => {
      this.#hear(message.data);
    });
    this.#relay({ kind: "hello" });
    void this.#campaign(false);
    setInterval(() => {
      this.#checkLeader();
    }, CHECK_MS);
  }

  public get status(): FeedStatus {
    return this.#published;
  }

  public get leading(): boolean {
    return this.#leading;
  }

  public onUpdate(listener: (update: LiveUpdate) => void): () => void {
    this.#updateListeners.add(listener);
    return () => {
      this.#updateListeners.delete(listener);
    };
  }

  public onStatus(listener: () => void): () => void {
    this.#statusListeners.add(listener);
    return () => {
      this.#statusListeners.delete(listener);
    };
  }

  /** Asks for the lock: queued behind the leader, or (`steal`) taking it from a frozen one. */
  async #campaign(steal: boolean): Promise<void> {
    const queued = new AbortController();
    this.#queued = queued;
    let granted = false;
    try {
      await this.#env.locks?.request(LOCK, steal ? { steal: true } : { signal: queued.signal }, async () => {
        granted = true;
        await this.#lead();
      });
    } catch {
      // Withdrawn while queued (this tab is stealing), or stolen while held.
    }
    // Only the request that held the lock can lose it: a withdrawn one settles after the steal was granted.
    if (granted && this.#leading) {
      // Stolen: stop streaming (or there would be two streams), and queue again; the new leader
      // gets a full wait before this tab would think it frozen.
      this.#run?.abort();
      this.#leading = false;
      this.#lastHeard = this.#env.now();
      this.#setStatus({ ...this.#status, ready: false });
      // The new leader answers with its state.
      this.#relay({ kind: "hello" });
      void this.#campaign(false);
    }
  }

  async #lead(): Promise<void> {
    this.#leading = true;
    const run = new AbortController();
    this.#run = run;
    // Not live until this tab's own stream says `ready` (it replays the backlog first); the
    // followers hear it at once, so none thinks the stream unattended.
    this.#setStatus({ ...this.#status, ready: false });
    this.#relay({ kind: "state", ...this.#status });
    const pulse = setInterval(() => {
      this.#relay({ kind: "alive" });
    }, PULSE_MS);
    try {
      await this.#env.follow({
        cursor: () => this.#env.cursor.read(),
        onUpdate: (update) => {
          // Relayed as received, applied here (a channel never delivers to its sender), then the cursor saved.
          this.#relay({ kind: "update", update });
          this.#highest = update.cursor ?? this.#highest;
          this.#receive(update);
          if (update.change?.kind === "reset") {
            // The cursor went back (a database reset): the stored one is from another timeline.
            this.#env.cursor.drop();
          }
          this.#env.cursor.save(update.cursor);
        },
        onDown: () => {
          this.#relay({ kind: "down" });
          this.#setStatus({ ...this.#status, ready: false });
        },
        dropCursor: () => {
          this.#env.cursor.drop();
        },
        signal: run.signal,
      });
    } finally {
      clearInterval(pulse);
    }
  }

  #relay(relay: Relay): void {
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel, not a window
    this.#env.channel?.postMessage(relay);
  }

  #hear(relay: Relay): void {
    // Another follower's `hello` says nothing of the leader.
    if (relay.kind !== "hello") {
      this.#lastHeard = this.#env.now();
    }
    if (relay.kind === "hello" && this.#leading) {
      this.#relay({ kind: "state", ...this.#status });
    } else if (relay.kind === "update" && !this.#leading && this.#isCurrent(relay.update)) {
      this.#receive(relay.update);
    } else if (relay.kind === "down" && !this.#leading) {
      this.#setStatus({ ...this.#status, ready: false });
    } else if (relay.kind === "state" && !this.#leading) {
      this.#setStatus({ ready: relay.ready, lastActivity: relay.lastActivity });
    }
  }

  /** Whether a relayed update is newer than any yet: a thawed ex-leader may send a few old ones. */
  #isCurrent({ change, cursor }: LiveUpdate): boolean {
    // `ready` and `reset` start a connection's run of cursors, which may go back (a database reset).
    const fresh = change?.kind === "ready" || change?.kind === "reset" || isAfter(cursor, this.#highest);
    if (fresh) {
      this.#highest = cursor;
    }
    return fresh;
  }

  #receive(update: LiveUpdate): void {
    const ready = update.change?.kind === "ready" ? true : this.#status.ready;
    this.#status = { ready, lastActivity: this.#env.now() };
    for (const listener of this.#updateListeners) {
      listener(update);
    }
    this.#notify();
  }

  #setStatus(status: FeedStatus): void {
    this.#status = status;
    this.#notify();
  }

  #notify(): void {
    const status = this.#status;
    const shown = this.#published;
    const quiet =
      status.ready === shown.ready &&
      status.lastActivity !== undefined &&
      shown.lastActivity !== undefined &&
      status.lastActivity - shown.lastActivity < PUBLISH_EVERY_MS;
    if (quiet) {
      return;
    }
    this.#published = status;
    for (const listener of this.#statusListeners) {
      listener();
    }
  }

  /**
   * A follower in view that hasn't heard the leader for `SILENT_MS` asks after it, and takes the
   * stream only if no answer comes: a follower waking from its own sleep finds the leader well.
   */
  #checkLeader(): void {
    const asked = this.#env.now();
    if (this.#leading || this.#probing || asked - this.#lastHeard < SILENT_MS || !this.#env.visible()) {
      return;
    }
    this.#probing = true;
    this.#relay({ kind: "hello" });
    setTimeout(() => {
      this.#probing = false;
      if (!this.#leading && this.#lastHeard < asked) {
        this.#lastHeard = this.#env.now();
        this.#queued?.abort();
        void this.#campaign(true);
      }
    }, PROBE_MS);
  }
}

export type { Channel, FeedEnv, FeedStatus, Locks, Relay };
export { FeedNode };

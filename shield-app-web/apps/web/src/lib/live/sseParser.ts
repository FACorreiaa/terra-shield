/**
 * The WHATWG event-stream format, parsed by hand (ServerSentEvents.swift). The browser's
 * `EventSource` can't be used: `/v1/stream`'s 20 s heartbeat is an `id:`-only block, which
 * dispatches nothing there, so a quiet stream would look dead; and it hides the status and
 * `Retry-After` of a refused connection.
 */

/** One dispatched event. */
interface SSEMessage {
  id: string | undefined;
  event: string;
  data: string;
}

/** Fed one line at a time, terminators removed. `lastEventId` moves on every `id:`, the heartbeat's too. */
class SSEParser {
  public lastEventId: string | undefined;
  /** The server's reconnection delay (`retry:`), in milliseconds. */
  public retryMs: number | undefined;
  #event = "";
  #data: string[] = [];
  #sawData = false;

  public constructor(lastEventId?: string) {
    this.lastEventId = lastEventId;
  }

  /** A message when `line` is the blank line that ends one with data. */
  public feed(line: string): SSEMessage | undefined {
    if (line === "") {
      return this.#dispatch();
    }
    if (line.startsWith(":")) {
      return undefined;
    }
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    const raw = colon === -1 ? "" : line.slice(colon + 1);
    this.#take(field, raw.startsWith(" ") ? raw.slice(1) : raw);
    return undefined;
  }

  #take(field: string, value: string): void {
    if (field === "event") {
      this.#event = value;
    } else if (field === "data") {
      this.#data.push(value);
      this.#sawData = true;
    } else if (field === "id" && !value.includes("\0")) {
      // An id containing NUL is ignored; an empty one resets it.
      this.lastEventId = value === "" ? undefined : value;
    } else if (field === "retry" && /^\d+$/u.test(value)) {
      this.retryMs = Number(value);
    }
  }

  #dispatch(): SSEMessage | undefined {
    const message = this.#sawData
      ? { id: this.lastEventId, event: this.#event === "" ? "message" : this.#event, data: this.#data.join("\n") }
      : undefined;
    this.#event = "";
    this.#data = [];
    this.#sawData = false;
    return message;
  }
}

export type { SSEMessage };
export { SSEParser };

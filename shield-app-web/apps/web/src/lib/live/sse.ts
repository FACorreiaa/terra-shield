import type { SSEMessage } from "./sseParser.ts";
import { SSEParser } from "./sseParser.ts";

/** A message, or an `id:`-only block: the heartbeat, which moves the cursor and proves the stream alive. */
type SSEOutput = { kind: "message"; message: SSEMessage } | { kind: "cursor"; id: string };

const LINE_END = /\r\n|\r|\n/u;

/** Text in (already decoded, in chunks of any size), messages and heartbeats out. */
class SSEStreamDecoder {
  readonly #parser: SSEParser;
  #line = "";
  /** The last chunk ended in CR: a LF opening the next one belongs to it. */
  #afterCR = false;
  /** The current block had an `id:` field. */
  #blockHasId = false;

  public constructor(lastEventId?: string) {
    this.#parser = new SSEParser(lastEventId);
  }

  public get lastEventId(): string | undefined {
    return this.#parser.lastEventId;
  }

  public feed(chunk: string): SSEOutput[] {
    if (chunk === "") {
      return [];
    }
    const text = this.#afterCR && chunk.startsWith("\n") ? chunk.slice(1) : chunk;
    const lines = (this.#line + text).split(LINE_END);
    this.#line = lines.pop() ?? "";
    this.#afterCR = text.endsWith("\r");
    return lines.flatMap((line) => this.#endLine(line) ?? []);
  }

  #endLine(line: string): SSEOutput | undefined {
    if (line === "id" || line.startsWith("id:")) {
      this.#blockHasId = true;
    }
    const message = this.#parser.feed(line);
    if (message !== undefined) {
      this.#blockHasId = false;
      return { kind: "message", message };
    }
    if (line !== "") {
      return undefined;
    }
    // Reported even when the id repeats (nothing changed): it is also the proof the stream lives.
    const heartbeat = this.#blockHasId ? this.#parser.lastEventId : undefined;
    this.#blockHasId = false;
    return heartbeat === undefined ? undefined : { kind: "cursor", id: heartbeat };
  }
}

export type { SSEOutput };
export { SSEStreamDecoder };

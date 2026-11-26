import { describe, expect, it } from "vite-plus/test";

import type { SSEOutput } from "./sse.ts";
import { SSEStreamDecoder } from "./sse.ts";
import type { SSEMessage } from "./sseParser.ts";
import { SSEParser } from "./sseParser.ts";

function messages(text: string, parser: SSEParser): SSEMessage[] {
  return text.split("\n").flatMap((line) => parser.feed(line) ?? []);
}

describe(SSEParser, () => {
  it("parses the shapes the server sends", () => {
    const parser = new SSEParser();
    const out = messages(
      'retry: 5000\n\nid: 41\nevent: event.updated\ndata: {"id":"a"}\n\nid: 42\n\nid: 43\nevent: ready\ndata: {}\n\n',
      parser,
    );
    expect(out).toStrictEqual([
      { id: "41", event: "event.updated", data: '{"id":"a"}' },
      { id: "43", event: "ready", data: "{}" },
    ]);
    expect(parser.retryMs).toBe(5000);
    expect(parser.lastEventId).toBe("43");
  });

  it("moves the cursor on a heartbeat without dispatching", () => {
    const parser = new SSEParser("7");
    expect(messages("id: 12\n\n", parser)).toStrictEqual([]);
    expect(parser.lastEventId).toBe("12");
  });

  it("follows the spec for edge cases", () => {
    const parser = new SSEParser();
    const out = messages(
      ": a comment\ndata:no space\ndata:  two spaces keep one\ndata\n\nevent: custom\ndata: x\nid: bad\0id\n\n",
      parser,
    );
    expect(out).toStrictEqual([
      { id: undefined, event: "message", data: "no space\n two spaces keep one\n" },
      { id: undefined, event: "custom", data: "x" },
    ]);
    // An id with NUL is ignored.
    expect(parser.lastEventId).toBeUndefined();
  });

  it("doesn't dispatch an event without data, nor leak its name", () => {
    const parser = new SSEParser();
    expect(messages("event: orphan\n\ndata: y\n\n", parser)).toStrictEqual([
      { id: undefined, event: "message", data: "y" },
    ]);
  });
});

function run(chunks: readonly string[]): SSEOutput[] {
  const decoder = new SSEStreamDecoder("1");
  return chunks.flatMap((chunk) => decoder.feed(chunk));
}

describe(SSEStreamDecoder, () => {
  const ready: SSEOutput = { kind: "message", message: { id: "3", event: "ready", data: "{}" } };

  it("ends lines at LF, CRLF and bare CR", () => {
    expect(run(["id: 3\nevent: ready\ndata: {}\n\n"])).toStrictEqual([ready]);
    expect(run(["id: 3\r\nevent: ready\r\ndata: {}\r\n\r\n"])).toStrictEqual([ready]);
    expect(run(["id: 3\revent: ready\rdata: {}\r\r"])).toStrictEqual([ready]);
    // A CRLF split across chunks is one line end, not two.
  });

  it("takes chunks split anywhere, a CRLF across two too", () => {
    expect(run(["id: 3\r", "\nevent: ready\r", "\ndata: {}\r", "\n\r", "\n"])).toStrictEqual([ready]);
    const text = "id: 3\nevent: ready\ndata: {}\n\n";
    expect(run(Array.from({ length: text.length }, (_unused, index) => text.charAt(index)))).toStrictEqual([ready]);
  });

  it("reports every heartbeat, a repeated id too, but not a lone retry", () => {
    expect(run(["id: 7\n\nid: 7\n\n"])).toStrictEqual([
      { kind: "cursor", id: "7" },
      { kind: "cursor", id: "7" },
    ]);
    expect(run(["retry: 5000\n\n"])).toStrictEqual([]);
  });

  it("keeps the cursor it started with until the stream moves it", () => {
    expect(run(["data: Évora\n\n"])).toStrictEqual([
      { kind: "message", message: { id: "1", event: "message", data: "Évora" } },
    ]);
  });
});

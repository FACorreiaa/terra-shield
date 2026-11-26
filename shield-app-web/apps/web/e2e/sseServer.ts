// oxlint-disable import/no-nodejs-modules, promise/avoid-new -- a server the test runner hosts, in Node
import type { ServerResponse } from "node:http";
import { createServer } from "node:http";

import type { BrowserContext, Page } from "@playwright/test";

/** A `/v1/stream` the test speaks through: Playwright's `fulfill` sends a whole body and closes, a stream never does. */
interface SseServer {
  /** Answers the page's `/v1/stream` from here. */
  serve: (target: Pick<Page | BrowserContext, "route">) => Promise<void>;
  /** Writes `block` (one or more SSE blocks) to every open connection. */
  send: (block: string) => void;
  /** Connections open now. */
  open: () => number;
  /** Each connection's `Last-Event-ID`, in order. */
  cursors: (string | undefined)[];
  close: () => Promise<void>;
}

const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "Accept, Cache-Control, Last-Event-ID",
};

async function sseServer(): Promise<SseServer> {
  const connections = new Set<ServerResponse>();
  const cursors: (string | undefined)[] = [];
  const server = createServer((request, response) => {
    if (request.method === "OPTIONS") {
      response.writeHead(204, CORS).end();
      return;
    }
    const cursor = request.headers["last-event-id"];
    cursors.push(typeof cursor === "string" ? cursor : undefined);
    response.writeHead(200, { ...CORS, "Content-Type": "text/event-stream", "Cache-Control": "no-store" });
    response.write("retry: 5000\n\n");
    connections.add(response);
    request.on("close", () => {
      connections.delete(response);
    });
  });
  await new Promise<void>((resolve) => {
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  const port = typeof address === "object" && address !== null ? address.port : 0;
  return {
    serve: async (target) => {
      await target.route("**/v1/stream", (route) => route.continue({ url: `http://127.0.0.1:${port}/v1/stream` }));
    },
    send: (block) => {
      for (const response of connections) {
        response.write(block);
      }
    },
    open: () => connections.size,
    cursors,
    close: async () => {
      for (const response of connections) {
        response.end();
      }
      server.closeAllConnections();
      await new Promise<void>((resolve) => {
        server.close(() => {
          resolve();
        });
      });
    },
  };
}

/** An SSE block for one event change. */
function change(id: number, name: string, data: unknown): string {
  return `id: ${id}\nevent: ${name}\ndata: ${JSON.stringify(data)}\n\n`;
}

export type { SseServer };
export { change, sseServer };

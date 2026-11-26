import type { HazardEvent } from "@shield/api-client";

import type { SSEMessage } from "./sseParser.ts";

/** A change from `GET /v1/stream` (ADR 16), decoded (ServerSentEvents.swift `LiveChange`). */
type LiveChange =
  /** `event.created`: applied like an upsert; once the stream is live it is also news (an arrival). */
  | { kind: "created"; event: HazardEvent }
  /** `event.updated` or `event.resolved`: the event in its latest state. */
  | { kind: "upsert"; event: HazardEvent }
  /** `event.removed`: retracted by its source, or deleted by retention. */
  | { kind: "removed"; id: string }
  /** The cursor is too far behind, too old or unknown: reload `/v1/events`. */
  | { kind: "reset" }
  /** Caught up; the stream continues live from here. */
  | { kind: "ready" };

/** One step of the stream: a change, or (the 20 s heartbeat) only a new cursor. */
interface LiveUpdate {
  change: LiveChange | undefined;
  /** Where to resume (`Last-Event-ID`). Compare, never count: values have gaps. */
  cursor: string | undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parsed(data: string): unknown {
  try {
    return JSON.parse(data) as unknown;
  } catch {
    return undefined;
  }
}

/** The fields the map relies on; the rest is as `/v1/events` sends it (same schema, without geometry). */
function isEvent(value: unknown): value is HazardEvent {
  if (!isRecord(value)) {
    return false;
  }
  const { centroid } = value;
  const strings = ["id", "type", "kind", "status", "severity", "title", "source", "updatedAt"] as const;
  return (
    strings.every((key) => typeof value[key] === "string") &&
    Array.isArray(value["countries"]) &&
    isRecord(centroid) &&
    typeof centroid["latitude"] === "number" &&
    typeof centroid["longitude"] === "number"
  );
}

function eventChange(message: SSEMessage): LiveChange | undefined {
  const event = parsed(message.data);
  if (!isEvent(event)) {
    return undefined;
  }
  return message.event === "event.created" ? { kind: "created", event } : { kind: "upsert", event };
}

function removal(message: SSEMessage): LiveChange | undefined {
  const body = parsed(message.data);
  const id = isRecord(body) ? body["id"] : undefined;
  return typeof id === "string" && id !== "" ? { kind: "removed", id } : undefined;
}

const DECODERS: Readonly<Record<string, (message: SSEMessage) => LiveChange | undefined>> = {
  "event.created": eventChange,
  "event.updated": eventChange,
  "event.resolved": eventChange,
  "event.removed": removal,
  reset: () => ({ kind: "reset" }),
  ready: () => ({ kind: "ready" }),
};

/**
 * The change a message carries; undefined for an unknown name or an undecodable payload. The cursor
 * still moves past it: the 60 s poll beside the stream picks the event up.
 */
function liveChange(message: SSEMessage): LiveChange | undefined {
  return Object.hasOwn(DECODERS, message.event) ? DECODERS[message.event]?.(message) : undefined;
}

export type { LiveChange, LiveUpdate };
export { liveChange };

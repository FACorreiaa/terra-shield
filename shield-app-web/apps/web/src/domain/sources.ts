/** Each source's health dot and lines on the Sources screen (DESIGN.md §10.7). Mirrors shield-core `Source`. */
import type { Source } from "@shield/api-client";

import { isStale as isOld, relative, stamp } from "./freshness.ts";

/** The dot on a source's row: `ok`, `warning` (the last check failed) or `danger` (stale). */
type SourceHealth = "ok" | "warning" | "danger";

const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;

function date(raw: string | undefined): Date | undefined {
  if (raw === undefined) {
    return undefined;
  }
  const parsed = new Date(raw);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed;
}

/** The sources the screen lists: every active one, in the API's order. */
function listed(sources: readonly Source[]): Source[] {
  return sources.filter((source) => source.enabled);
}

/**
 * Uses the server's `stale` when present, else the same rule locally: an on-demand source is
 * never stale, and one without a success is.
 */
function isStale(source: Source, now: Date = new Date()): boolean {
  if (source.stale !== undefined) {
    return source.stale;
  }
  if (source.onDemand === true) {
    return false;
  }
  const lastSuccess = date(source.lastSuccessAt);
  return lastSuccess === undefined || isOld(lastSuccess, source.pollIntervalSeconds, now);
}

/** An error counts only when it is newer than the last success. */
function lastCheckFailed(source: Source): boolean {
  const lastError = date(source.lastErrorAt);
  if (lastError === undefined) {
    return false;
  }
  const lastSuccess = date(source.lastSuccessAt);
  return lastSuccess === undefined || lastError > lastSuccess;
}

/**
 * EONET failed at 17:03 and recovered at 17:41 on 2026-11-19: healthy. Stale, by the server's
 * rule, outranks a failed check.
 */
function health(source: Source, now: Date = new Date()): SourceHealth {
  if (isStale(source, now)) {
    return "danger";
  }
  return lastCheckFailed(source) ? "warning" : "ok";
}

/**
 * "Checked 2 min ago · 17:38 UTC", "Last check failed 3 min ago · last good check 1 h ago",
 * "Stale · last good check 19 h ago · 22:40 UTC".
 */
function statusLine(source: Source, now: Date = new Date()): string {
  const lastError = date(source.lastErrorAt);
  const lastSuccess = date(source.lastSuccessAt);
  if (lastCheckFailed(source) && lastError !== undefined) {
    const good = lastSuccess === undefined ? "no good check yet" : `last good check ${relative(lastSuccess, now)}`;
    return `Last check failed ${relative(lastError, now)} · ${good}`;
  }
  if (lastSuccess === undefined) {
    return "No good check yet";
  }
  if (isStale(source, now)) {
    return `Stale · last good check ${stamp(lastSuccess, now)}`;
  }
  return `Checked ${stamp(lastSuccess, now)}`;
}

/** "30 s", "5 min", "6 h". */
function every(seconds: number): string {
  if (seconds < SECONDS_PER_MINUTE) {
    return `${seconds} s`;
  }
  if (seconds % SECONDS_PER_HOUR === 0) {
    return `${seconds / SECONDS_PER_HOUR} h`;
  }
  return `${Math.round(seconds / SECONDS_PER_MINUTE)} min`;
}

/**
 * Our poll, not the data's age: "Checked every 6 h", or with the source's own rhythm,
 * "Monthly release · checked every 6 h".
 */
function interval(source: Pick<Source, "pollIntervalSeconds" | "cadence">): string {
  const poll = every(source.pollIntervalSeconds);
  const { cadence } = source;
  if (cadence === undefined || cadence === "") {
    return `Checked every ${poll}`;
  }
  return `${cadence.charAt(0).toUpperCase()}${cadence.slice(1)} · checked every ${poll}`;
}

export type { SourceHealth };
export { health, interval, isStale, lastCheckFailed, listed, statusLine };

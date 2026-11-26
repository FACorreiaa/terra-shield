/**
 * The status capsule's words and colours (DESIGN.md §10.1 states). Mirrors the apps' `StatusCapsule`
 * over `LiveMapModel.phase` and `dataIsStale`.
 */
import { relative, utcClock } from "./freshness.ts";

/** How the events load is going. */
interface LoadState {
  /** When the events last arrived; undefined before the first success. */
  lastSuccess: Date | undefined;
  /** A request is in flight. */
  loading: boolean;
  /** The browser is offline, so requests wait instead of failing. */
  offline: boolean;
  /** The last request failed while online. */
  failed: boolean;
  /**
   * While the live stream is connected and caught up (W6), when it last said anything, a heartbeat
   * included: the data was confirmed current then. Undefined while only polling.
   */
  liveAt: Date | undefined;
  now: Date;
}

type Tone = "ok" | "warning" | "danger" | "muted";

interface LiveStatus {
  text: string;
  /** The dot's colour. */
  dot: Tone;
  /** The words are `warning` when stale or offline, else ink. */
  warns: boolean;
  /** Offline: the capsule carries its own Retry. */
  retry: boolean;
  /** The live stream is connected: the dot fills (a ring while only polling). */
  live: boolean;
}

const MS_PER_SECOND = 1000;
/** The map refreshes every 60 s; three missed refreshes make the data stale (LiveMapModel). */
const REFRESH_SECONDS = 60;
const MISSED_REFRESHES = 3;
const STALE_AFTER_MS = MISSED_REFRESHES * REFRESH_SECONDS * MS_PER_SECOND;

/** The map hasn't refreshed for three refresh intervals; never while live, the stream keeps it current. */
function isDataStale({ lastSuccess, liveAt, now }: Pick<LoadState, "lastSuccess" | "liveAt" | "now">): boolean {
  return liveAt === undefined && lastSuccess !== undefined && now.getTime() - lastSuccess.getTime() > STALE_AFTER_MS;
}

/** When the data was last known current: a load, or while live the stream's latest word (LiveMapModel `lastUpdate`). */
function lastUpdate({ lastSuccess, liveAt }: Pick<LoadState, "lastSuccess" | "liveAt">): Date | undefined {
  if (liveAt === undefined || (lastSuccess !== undefined && lastSuccess > liveAt)) {
    return lastSuccess;
  }
  return liveAt;
}

function offlineStatus(lastSuccess: Date | undefined): LiveStatus {
  const text = lastSuccess === undefined ? "Offline" : `Offline · showing data from ${utcClock(lastSuccess)}`;
  return { text, dot: "muted", warns: true, retry: true, live: false };
}

/** "Loading events…", "Offline · showing data from 14:02 UTC", "Updated 2 min ago" or "Last update 14 min ago". */
function liveStatus(state: LoadState): LiveStatus {
  const { lastSuccess, loading, offline, failed, liveAt, now } = state;
  if (offline) {
    return offlineStatus(lastSuccess);
  }
  const live = liveAt !== undefined;
  const last = lastUpdate(state);
  if (last === undefined || (lastSuccess === undefined && loading)) {
    return {
      text: loading ? "Loading events…" : "No data yet",
      dot: failed ? "danger" : "ok",
      warns: false,
      retry: false,
      live,
    };
  }
  const stale = isDataStale(state);
  const ago = relative(last, now);
  const healthy: Tone = stale ? "warning" : "ok";
  return {
    text: stale ? `Last update ${ago}` : `Updated ${ago}`,
    dot: failed ? "danger" : healthy,
    warns: stale,
    retry: false,
    live,
  };
}

export type { LiveStatus, LoadState, Tone };
export { isDataStale, liveStatus };

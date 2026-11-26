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
}

const MS_PER_SECOND = 1000;
/** The map refreshes every 60 s; three missed refreshes make the data stale (LiveMapModel). */
const REFRESH_SECONDS = 60;
const MISSED_REFRESHES = 3;
const STALE_AFTER_MS = MISSED_REFRESHES * REFRESH_SECONDS * MS_PER_SECOND;

function isDataStale({ lastSuccess, now }: Pick<LoadState, "lastSuccess" | "now">): boolean {
  return lastSuccess !== undefined && now.getTime() - lastSuccess.getTime() > STALE_AFTER_MS;
}

function offlineStatus(lastSuccess: Date | undefined): LiveStatus {
  const text = lastSuccess === undefined ? "Offline" : `Offline · showing data from ${utcClock(lastSuccess)}`;
  return { text, dot: "muted", warns: true, retry: true };
}

/** "Loading events…", "Offline · showing data from 14:02 UTC", "Updated 2 min ago" or "Last update 14 min ago". */
function liveStatus(state: LoadState): LiveStatus {
  const { lastSuccess, loading, offline, failed, now } = state;
  if (offline) {
    return offlineStatus(lastSuccess);
  }
  if (lastSuccess === undefined) {
    return {
      text: loading ? "Loading events…" : "No data yet",
      dot: failed ? "danger" : "ok",
      warns: false,
      retry: false,
    };
  }
  const stale = isDataStale(state);
  const ago = relative(lastSuccess, now);
  const healthy: Tone = stale ? "warning" : "ok";
  return {
    text: stale ? `Last update ${ago}` : `Updated ${ago}`,
    dot: failed ? "danger" : healthy,
    warns: stale,
    retry: false,
  };
}

export type { LiveStatus, LoadState, Tone };
export { isDataStale, liveStatus };

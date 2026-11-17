/** Freshness wording shared by every surface (DESIGN.md §2, DECISIONS #8). Mirrors shield-core. */

const MS_PER_SECOND = 1000;
const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86_400;
/** Below this, a timestamp reads "now". */
const NOW_SECONDS = 10;
/** Stale after this many missed poll intervals. */
const STALE_AFTER_POLLS = 3;

/** Stale when the last update is older than three poll intervals. */
function isStale(lastUpdate: Date, pollIntervalSeconds: number, now: Date = new Date()): boolean {
  return now.getTime() - lastUpdate.getTime() > STALE_AFTER_POLLS * pollIntervalSeconds * MS_PER_SECOND;
}

function span(seconds: number): string {
  if (seconds < SECONDS_PER_MINUTE) {
    return `${Math.floor(seconds)} s`;
  }
  if (seconds < SECONDS_PER_HOUR) {
    return `${Math.floor(seconds / SECONDS_PER_MINUTE)} min`;
  }
  if (seconds < SECONDS_PER_DAY) {
    return `${Math.floor(seconds / SECONDS_PER_HOUR)} h`;
  }
  return `${Math.floor(seconds / SECONDS_PER_DAY)} d`;
}

/** "now", "45 s ago", "12 min ago", "3 h ago", "2 d ago"; future dates read "in 12 min". */
function relative(date: Date, now: Date = new Date()): string {
  const delta = (now.getTime() - date.getTime()) / MS_PER_SECOND;
  if (Math.abs(delta) < NOW_SECONDS) {
    return "now";
  }
  return delta < 0 ? `in ${span(-delta)}` : `${span(delta)} ago`;
}

/** "14:02 UTC". */
function utcClock(date: Date): string {
  const hh = String(date.getUTCHours()).padStart(2, "0");
  const mm = String(date.getUTCMinutes()).padStart(2, "0");
  return `${hh}:${mm} UTC`;
}

export { isStale, relative, utcClock };

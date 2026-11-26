import type { HazardEvent } from "@shield/api-client";

/** Freshness wording shared by every surface (DESIGN.md §2, §5; DECISIONS #8). Mirrors shield-core. */

const MS_PER_SECOND = 1000;
const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86_400;
/** Below this, a timestamp reads "now". */
const NOW_SECONDS = 10;
/** Stale after this many missed poll intervals. */
const STALE_AFTER_POLLS = 3;
/** Clock fields are two digits: "08:05". */
const CLOCK_DIGITS = 2;
/** en-GB abbreviated months, fixed so ICU's "Sept" never differs from the apps' "Sep". */
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;

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
  const hh = String(date.getUTCHours()).padStart(CLOCK_DIGITS, "0");
  const mm = String(date.getUTCMinutes()).padStart(CLOCK_DIGITS, "0");
  return `${hh}:${mm} UTC`;
}

/** "12 min ago · 14:02 UTC". */
function label(date: Date, now: Date = new Date()): string {
  return `${relative(date, now)} · ${utcClock(date)}`;
}

/** "13 Nov", in UTC: a day without a time. */
function utcDay(date: Date): string {
  return `${date.getUTCDate()} ${MONTHS[date.getUTCMonth()] ?? ""}`;
}

/** As `label`, with the date once more than a day away: "3 d ago · 13 Nov, 14:20 UTC". */
function stamp(date: Date, now: Date = new Date()): string {
  if (Math.abs(now.getTime() - date.getTime()) < SECONDS_PER_DAY * MS_PER_SECOND) {
    return label(date, now);
  }
  return `${relative(date, now)} · ${utcDay(date)}, ${utcClock(date)}`;
}

/**
 * "Resolved · ended 3 h ago": the end time when the source gave one that has passed, else the last
 * update ("Resolved · updated 20 min ago"); undefined for an active event (DESIGN.md §5, §10.6).
 */
function resolvedWords(
  event: Pick<HazardEvent, "status" | "endedAt" | "updatedAt">,
  now: Date = new Date(),
): string | undefined {
  if (event.status !== "resolved") {
    return undefined;
  }
  if (event.endedAt !== undefined) {
    const ended = new Date(event.endedAt);
    if (ended.getTime() <= now.getTime()) {
      return `Resolved · ended ${relative(ended, now)}`;
    }
  }
  return `Resolved · updated ${relative(new Date(event.updatedAt), now)}`;
}

export { isStale, label, relative, resolvedWords, stamp, utcClock, utcDay };

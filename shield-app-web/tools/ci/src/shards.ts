/**
 * Splits test files into `count` shards of near-equal total time (longest-processing-time first:
 * place each file, slowest first, on the currently lightest shard). Deterministic for the same
 * input, so every CI shard computes the same plan independently.
 */
interface TimedFile {
  /** Path relative to the workspace root, with forward slashes. */
  path: string;
  /** Recorded milliseconds; absent for a file with no record yet. */
  durationMs?: number | undefined;
}

interface Shard {
  files: string[];
  totalMs: number;
}

/** Assumed cost of a test file when nothing at all has been recorded. */
const DEFAULT_ESTIMATE_MS = 1000;

/** Fallback for files with no record: the median of what is known. */
function estimateFor(known: readonly number[]): number {
  const sorted = known.toSorted((left, right) => left - right);
  const mid = Math.floor(sorted.length / 2);
  const upper = sorted[mid] ?? DEFAULT_ESTIMATE_MS;
  const lower = sorted[mid - 1] ?? upper;
  return sorted.length % 2 === 0 ? (lower + upper) / 2 : upper;
}

function lightestIndex(shards: readonly Shard[]): number {
  let lightest = 0;
  for (const [index, shard] of shards.entries()) {
    if (shard.totalMs < (shards[lightest]?.totalMs ?? 0)) {
      lightest = index;
    }
  }
  return lightest;
}

function planShards(files: readonly TimedFile[], count: number): Shard[] {
  if (!Number.isInteger(count) || count < 1) {
    throw new RangeError(`shard count must be a positive integer, got ${count}`);
  }
  const fallback = estimateFor(files.flatMap((file) => (file.durationMs === undefined ? [] : [file.durationMs])));
  const weighted = files
    .map((file) => ({ path: file.path, ms: file.durationMs ?? fallback }))
    .toSorted((left, right) => right.ms - left.ms || left.path.localeCompare(right.path));

  const shards: Shard[] = Array.from({ length: count }, () => ({ files: [], totalMs: 0 }));
  for (const file of weighted) {
    const shard = shards[lightestIndex(shards)];
    if (shard !== undefined) {
      shard.files.push(file.path);
      shard.totalMs += file.ms;
    }
  }
  return shards;
}

export type { Shard, TimedFile };
export { estimateFor, planShards };

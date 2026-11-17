import { readdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import type { Timings } from "./timings.ts";
import { readTimings } from "./timings.ts";

/**
 * `bun tools/ci/src/merge.ts <dir-of-shard-files> <out>`: merges per-shard timings into one file.
 * Files no longer run (deleted tests) drop out, because only this run's shards are merged.
 */
function merge(dir: string, out: string): number {
  const merged: Timings = {};
  for (const name of readdirSync(dir)
    .filter((file) => file.endsWith(".json"))
    .toSorted()) {
    Object.assign(merged, readTimings(path.join(dir, name)));
  }
  const sorted = Object.fromEntries(Object.entries(merged).toSorted(([left], [right]) => left.localeCompare(right)));
  writeFileSync(out, `${JSON.stringify(sorted, undefined, 2)}\n`);
  return Object.keys(sorted).length;
}

const [dir, out] = process.argv.slice(2);
if (dir === undefined || out === undefined) {
  throw new Error("usage: merge.ts <dir> <out>");
}
process.stdout.write(`merged ${merge(dir, out)} test files into ${out}\n`);

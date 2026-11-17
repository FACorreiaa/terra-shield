import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

/** `{ "apps/web/src/x.test.ts": 812 }`, milliseconds per test file. */
type Timings = Record<string, number>;

/** Where recorded timings are read from; CI restores this file from its cache. */
function timingsFile(root: string): string {
  return path.resolve(root, process.env["TEST_TIMINGS_FILE"] ?? ".ci/test-timings.json");
}

function readTimings(file: string): Timings {
  if (!existsSync(file)) {
    return {};
  }
  const parsed: unknown = JSON.parse(readFileSync(file, "utf8"));
  if (typeof parsed !== "object" || parsed === null) {
    return {};
  }
  return Object.fromEntries(
    Object.entries(parsed).filter((entry): entry is [string, number] => typeof entry[1] === "number"),
  );
}

function workspacePath(root: string, file: string): string {
  return path.relative(root, file).split(path.sep).join("/");
}

export type { Timings };
export { readTimings, timingsFile, workspacePath };

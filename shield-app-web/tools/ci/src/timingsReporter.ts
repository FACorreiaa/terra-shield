import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import type { Reporter, TestModule, Vitest } from "vite-plus/test/node";

import type { Timings } from "./timings.ts";
import { workspacePath } from "./timings.ts";

/**
 * Records what each test file cost its shard (environment, setup, collect and run) to
 * `TEST_TIMINGS_OUT`. Off unless that variable is set, so local runs write nothing.
 * CI merges the per-shard files into `.ci/test-timings.json` (see `merge.ts`).
 */
export default class TimingsReporter implements Reporter {
  private root = process.cwd();

  public onInit(vitest: Vitest): void {
    this.root = vitest.config.root;
  }

  public onTestRunEnd(testModules: readonly TestModule[]): void {
    const out = process.env["TEST_TIMINGS_OUT"];
    if (out === undefined || out === "") {
      return;
    }
    const timings: Timings = {};
    for (const testModule of testModules) {
      const diag = testModule.diagnostic();
      const total =
        diag.environmentSetupDuration +
        diag.prepareDuration +
        diag.collectDuration +
        diag.setupDuration +
        diag.duration;
      timings[workspacePath(this.root, testModule.moduleId)] = Math.round(total);
    }
    const file = path.resolve(this.root, out);
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, `${JSON.stringify(timings, undefined, 2)}\n`);
  }
}

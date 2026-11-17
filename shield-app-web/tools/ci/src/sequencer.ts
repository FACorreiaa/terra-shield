import type { TestSpecification } from "vite-plus/test/node";
import { BaseSequencer } from "vite-plus/test/node";

import { planShards } from "./shards.ts";
import { readTimings, timingsFile, workspacePath } from "./timings.ts";

/**
 * Vitest's own `--shard` splits by hashed path into equal file counts. This one splits by
 * recorded time (`.ci/test-timings.json`, written by the timings reporter), so shards finish
 * together. Ordering inside a shard stays Vitest's (slowest and failed-last-time first).
 */
class TimeShardSequencer extends BaseSequencer {
  public override shard(files: TestSpecification[]): Promise<TestSpecification[]> {
    const { config } = this.ctx;
    const { index, count } = config.shard ?? { index: 1, count: 1 };
    const { root } = config;
    const timings = readTimings(timingsFile(root));
    const plan = planShards(
      files.map((spec) => {
        const file = workspacePath(root, spec.moduleId);
        return { path: file, durationMs: timings[file] };
      }),
      count,
    );
    const mine = new Set(plan[index - 1]?.files);
    return Promise.resolve(files.filter((spec) => mine.has(workspacePath(root, spec.moduleId))));
  }
}

export { TimeShardSequencer };

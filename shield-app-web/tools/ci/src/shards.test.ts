import { describe, expect, it } from "vite-plus/test";

import { estimateFor, planShards } from "./shards.ts";

describe(planShards, () => {
  it("balances by time, not by file count", () => {
    const plan = planShards(
      [
        { path: "slow.test.ts", durationMs: 9000 },
        { path: "a.test.ts", durationMs: 3000 },
        { path: "b.test.ts", durationMs: 3000 },
        { path: "c.test.ts", durationMs: 3000 },
      ],
      2,
    );
    expect(plan.map((shard) => shard.files)).toStrictEqual([["slow.test.ts"], ["a.test.ts", "b.test.ts", "c.test.ts"]]);
    expect(plan.map((shard) => shard.totalMs)).toStrictEqual([9000, 9000]);
  });

  it("gives every file to exactly one shard", () => {
    const files = Array.from({ length: 23 }, (_unused, index) => ({
      path: `t${index}.test.ts`,
      durationMs: (index * 37) % 500,
    }));
    const assigned = planShards(files, 4)
      .flatMap((shard) => shard.files)
      .toSorted();
    expect(assigned).toStrictEqual(files.map((file) => file.path).toSorted());
  });

  it("is deterministic for ties", () => {
    const files = [{ path: "b.test.ts" }, { path: "a.test.ts" }, { path: "c.test.ts" }];
    expect(planShards(files, 2)).toStrictEqual(planShards(files.toReversed(), 2));
  });

  it("estimates unrecorded files at the median", () => {
    const plan = planShards(
      [
        { path: "known1.test.ts", durationMs: 100 },
        { path: "known2.test.ts", durationMs: 300 },
        { path: "new.test.ts" },
      ],
      1,
    );
    expect(plan[0]?.totalMs).toBe(600);
  });

  it("rejects a bad shard count", () => {
    expect(() => planShards([], 0)).toThrow(RangeError);
  });
});

describe(estimateFor, () => {
  it("falls back to one second with no data, else the median", () => {
    expect(estimateFor([])).toBe(1000);
    expect(estimateFor([10, 20, 90])).toBe(20);
  });
});

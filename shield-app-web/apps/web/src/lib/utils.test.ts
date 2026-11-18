import { describe, expect, it } from "vite-plus/test";

import { cn } from "./utils.ts";

describe(cn, () => {
  it("keeps a token text size next to a text colour", () => {
    expect(cn("text-meta text-ink")).toBe("text-meta text-ink");
  });

  it("still resolves conflicts within a group", () => {
    expect(cn("text-meta", "text-body")).toBe("text-body");
    expect(cn("text-ink", "text-muted")).toBe("text-muted");
  });
});

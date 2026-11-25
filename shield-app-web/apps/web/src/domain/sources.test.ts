import type { Source } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import { health, interval, listed, statusLine } from "./sources.ts";

// 2026-11-19 17:40 UTC, as in shield-core SourcesTests.
const now = new Date(1_795_110_000_000);
const at = (seconds: number): string => new Date(now.getTime() + seconds * 1000).toISOString();

interface Options {
  poll?: number;
  success?: number;
  error?: number;
  stale?: boolean;
  enabled?: boolean;
  cadence?: string;
}

function source({ poll = 600, success, error, stale, enabled = true, cadence }: Options): Source {
  return {
    id: "eonet",
    name: "NASA EONET",
    url: "https://eonet.gsfc.nasa.gov/",
    license: "Public domain (NASA)",
    attribution: "NASA EONET",
    pollIntervalSeconds: poll,
    enabled,
    ...(success === undefined ? {} : { lastSuccessAt: at(success) }),
    ...(error === undefined ? {} : { lastErrorAt: at(error) }),
    ...(stale === undefined ? {} : { stale }),
    ...(cadence === undefined ? {} : { cadence }),
  };
}

describe(health, () => {
  it("ignores an error older than the last success", () => {
    // Live on 2026-11-19: EONET failed at 17:03 and succeeded since at 17:41.
    const eonet = source({ success: -120, error: -2400, stale: false });
    expect(health(eonet, now)).toBe("ok");
    expect(statusLine(eonet, now)).toBe("Checked 2 min ago · 17:38 UTC");
  });

  it("warns when the last check failed", () => {
    const eonet = source({ success: -3600, error: -180, stale: false });
    expect(health(eonet, now)).toBe("warning");
    expect(statusLine(eonet, now)).toBe("Last check failed 3 min ago · last good check 1 h ago");
  });

  it("says so when it never succeeded", () => {
    const eonet = source({ error: -180 });
    expect(health(eonet, now)).toBe("danger");
    expect(statusLine(eonet, now)).toBe("Last check failed 3 min ago · no good check yet");
    expect(statusLine(source({}), now)).toBe("No good check yet");
  });

  it("lets the server's stale verdict win", () => {
    const eonet = source({ success: -19 * 3600, stale: true });
    expect(health(eonet, now)).toBe("danger");
    expect(statusLine(eonet, now)).toBe("Stale · last good check 19 h ago · 22:40 UTC");
    const days = source({ success: -3 * 86_400, stale: true });
    expect(statusLine(days, now)).toBe("Stale · last good check 3 d ago · 16 Nov, 17:40 UTC");
  });

  it("applies the stale rule locally without the server's verdict", () => {
    expect(health(source({ success: -1801 }), now)).toBe("danger");
    expect(health(source({ success: -1799 }), now)).toBe("ok");
  });

  it("never calls an on-demand source stale", () => {
    expect(health({ ...source({}), onDemand: true }, now)).toBe("ok");
  });
});

describe(interval, () => {
  it.each([
    [60, "Checked every 1 min"],
    [300, "Checked every 5 min"],
    [21_600, "Checked every 6 h"],
    [30, "Checked every 30 s"],
  ])("reads a %i s poll as %s", (poll, expected) => {
    expect(interval(source({ poll }))).toBe(expected);
  });

  it("names the source's own rhythm first", () => {
    expect(interval(source({ poll: 21_600, cadence: "monthly release" }))).toBe("Monthly release · checked every 6 h");
  });
});

describe(listed, () => {
  it("lists only active sources", () => {
    expect(listed([source({}), source({ enabled: false })])).toHaveLength(1);
  });
});

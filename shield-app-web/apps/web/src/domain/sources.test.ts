import type { Source } from "@shield/api-client";
import { describe, expect, it } from "vite-plus/test";

import { health, interval, listed, overallHealth, statusLine } from "./sources.ts";

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
  onDemand?: boolean;
}

function source({ poll = 600, success, error, stale, enabled = true, cadence, onDemand }: Options): Source {
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
    ...(onDemand === undefined ? {} : { onDemand }),
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

describe(overallHealth, () => {
  const ok = source({ success: -120, stale: false });
  const failing = source({ success: -2400, error: -120, stale: false });
  const stale = source({ success: -86_400, stale: true });

  it("has no dot before the first list: no health is not good health", () => {
    expect(overallHealth(undefined, now)).toBeUndefined();
    expect(overallHealth([], now)).toBeUndefined();
  });

  it("shows the worst listed source: stale over a failed check over healthy", () => {
    expect(overallHealth([ok, ok], now)).toBe("ok");
    expect(overallHealth([ok, failing], now)).toBe("warning");
    expect(overallHealth([failing, stale, ok], now)).toBe("danger");
  });

  it("leaves out a disabled source, as the list does", () => {
    expect(overallHealth([ok, { ...stale, enabled: false }], now)).toBe("ok");
    expect(overallHealth([{ ...stale, enabled: false }], now)).toBeUndefined();
  });
});

describe("an on-demand source (live flights)", () => {
  it("says when it was fetched, never stale, and why it isn't polled", () => {
    const flights = source({ poll: 60, success: -10_800, onDemand: true });
    expect(health(flights, now)).toBe("ok");
    expect(statusLine(flights, now)).toBe("Fetched 3 h ago · 14:40 UTC");
    expect(statusLine(source({ poll: 60, onDemand: true }), now)).toBe("Not fetched yet");
    expect(interval(flights)).toBe("Fetched only while the flights layer is shown, at most once a minute");
  });

  it("still says when its last fetch failed", () => {
    const flights = source({ poll: 60, success: -600, error: -60, onDemand: true });
    expect(statusLine(flights, now)).toBe("Last check failed 1 min ago · last good check 10 min ago");
  });
});

describe("overallHealth without the server's stale", () => {
  it("applies the local rule: no success, or three missed polls, is stale", () => {
    expect(overallHealth([source({ success: -120 })], now)).toBe("ok");
    expect(overallHealth([source({ success: -120 }), source({})], now)).toBe("danger");
  });
});

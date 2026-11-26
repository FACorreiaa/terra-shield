import { describe, expect, it } from "vite-plus/test";

import { atRest, projected, rubberBand, snapTarget, springStep, stepDetent } from "./sheetPhysics.ts";

/** Peek, medium and large on an 844 px phone. */
const DETENTS = [96, 422, 776] as const;
const FRAME = 1 / 60;
const RANGE = { min: 96, max: 776 };

describe(snapTarget, () => {
  it("settles on the nearest detent when released still", () => {
    expect(snapTarget(300, 0, DETENTS)).toBe(422);
    expect(snapTarget(200, 0, DETENTS)).toBe(96);
  });

  it("follows a flick past the nearest detent", () => {
    // Released just above peek, flicked upward hard: it lands on large, not medium.
    expect(snapTarget(120, 1800, DETENTS)).toBe(776);
    // Released near large, flicked down: it drops past medium to peek.
    expect(snapTarget(700, -2200, DETENTS)).toBe(96);
  });

  it("a gentle flick moves one detent", () => {
    expect(snapTarget(430, 400, DETENTS)).toBe(776);
    expect(snapTarget(430, -400, DETENTS)).toBe(96);
  });
});

describe(projected, () => {
  it("carries a flick about half a second's travel", () => {
    expect(projected(0, 1000)).toBeCloseTo(499);
    expect(projected(100, 0)).toBe(100);
  });
});

describe(rubberBand, () => {
  it("leaves positions inside the range alone", () => {
    expect(rubberBand(300, RANGE, 844)).toBe(300);
  });

  it("resists more the further past the edge", () => {
    const small = rubberBand(786, RANGE, 844) - 776;
    const large = rubberBand(976, RANGE, 844) - 776;
    expect(small).toBeLessThan(10);
    expect(large).toBeLessThan(200);
    expect(large).toBeGreaterThan(small);
    expect(rubberBand(46, RANGE, 844)).toBeGreaterThan(46);
  });
});

function settle(from: number, to: number, velocity = 0): { frames: number; peak: number } {
  let state = { position: from, velocity };
  let frames = 0;
  let peak = from;
  while (!atRest(state, to) && frames < 600) {
    state = springStep(state, to, FRAME);
    peak = Math.max(peak, state.position);
    frames += 1;
  }
  return { frames, peak };
}

describe(springStep, () => {
  it("reaches the detent in well under a second", () => {
    expect(settle(96, 422).frames).toBeLessThan(45);
  });

  it("never overshoots more than slightly (damping 0.86)", () => {
    const { peak } = settle(96, 422, 2500);
    expect(peak - 422).toBeLessThan(0.05 * (422 - 96));
  });

  it("carries the release velocity into the motion", () => {
    const still = springStep({ position: 300, velocity: 0 }, 422, FRAME);
    const thrown = springStep({ position: 300, velocity: 1500 }, 422, FRAME);
    expect(thrown.position).toBeGreaterThan(still.position);
  });
});

describe(stepDetent, () => {
  it("moves one detent at a time and stops at the ends", () => {
    expect(stepDetent(96, DETENTS, 1)).toBe(422);
    expect(stepDetent(422, DETENTS, 1)).toBe(776);
    expect(stepDetent(776, DETENTS, 1)).toBe(776);
    expect(stepDetent(422, DETENTS, -1)).toBe(96);
    expect(stepDetent(96, DETENTS, -1)).toBe(96);
  });
});

/**
 * The bottom sheet's physics, kept pure for tests (DESIGN.md §10 shells, §13 motion). Heights are
 * the sheet's visible height in px, measured up from the bottom of the screen; velocities are px/s,
 * positive when the sheet grows.
 */

/** Apple's scroll deceleration: how far a flick carries before it would stop. */
const DECELERATION = 0.998;
const MS_PER_SECOND = 1000;
/** How hard the sheet resists past its top or bottom detent (iOS uses 0.55). */
const RUBBER_BAND = 0.55;

/** DESIGN.md §13 `spring`: response 0.35 s, damping 0.86. */
const SPRING = { response: 0.35, damping: 0.86 } as const;
const TURN = 2 * Math.PI;
/** Close enough to rest to stop animating. */
const REST = { distance: 0.5, velocity: 5 } as const;

interface SpringState {
  position: number;
  velocity: number;
}

/** Where a release at `velocity` would come to rest if nothing caught it (Apple's projection). */
function projected(position: number, velocity: number): number {
  return position + ((velocity / MS_PER_SECOND) * DECELERATION) / (1 - DECELERATION);
}

/** The detent nearest to where the gesture is going, not where it was released. */
function snapTarget(position: number, velocity: number, detents: readonly number[]): number {
  const landing = projected(position, velocity);
  let best = detents[0] ?? position;
  for (const detent of detents) {
    if (Math.abs(detent - landing) < Math.abs(best - landing)) {
      best = detent;
    }
  }
  return best;
}

interface Range {
  min: number;
  max: number;
}

/** A position past the range follows the finger less and less, so the edge feels soft. */
function rubberBand(position: number, { min, max }: Range, dimension: number): number {
  const band = (over: number): number => (over * dimension * RUBBER_BAND) / (dimension + RUBBER_BAND * over);
  if (position > max) {
    return max + band(position - max);
  }
  if (position < min) {
    return min - band(min - position);
  }
  return position;
}

/**
 * One step of a damped spring toward `target`, semi-implicit Euler. Apple's response/damping map to
 * stiffness (2π / response)² and damping 4π·ζ / response, for unit mass.
 */
function springStep(state: SpringState, target: number, seconds: number): SpringState {
  const stiffness = (TURN / SPRING.response) ** 2;
  const friction = (2 * TURN * SPRING.damping) / SPRING.response;
  const acceleration = -stiffness * (state.position - target) - friction * state.velocity;
  const velocity = state.velocity + acceleration * seconds;
  return { position: state.position + velocity * seconds, velocity };
}

function atRest(state: SpringState, target: number): boolean {
  return Math.abs(state.position - target) < REST.distance && Math.abs(state.velocity) < REST.velocity;
}

/** The next detent up or down from `current`, for the keyboard; it stays put at either end. */
function stepDetent(current: number, detents: readonly number[], direction: 1 | -1): number {
  const sorted = [...detents].toSorted((left, right) => left - right);
  const index = sorted.findIndex((detent) => detent >= current - REST.distance);
  const from = index === -1 ? sorted.length - 1 : index;
  return sorted[Math.min(sorted.length - 1, Math.max(0, from + direction))] ?? current;
}

export type { SpringState };
export { atRest, projected, rubberBand, snapTarget, springStep, stepDetent };

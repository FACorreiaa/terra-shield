import type { ReactElement } from "react";
import { useMemo } from "react";

interface ExtremeHaloProps {
  /** Side of the marker shape it surrounds, in px. */
  size: number;
}

/**
 * The extreme pulse (DESIGN.md §13): a ring in `severity.extreme` that grows and fades every
 * `extremePulse`. Under reduced motion it is a static double ring instead. Never takes input.
 */
function ExtremeHalo({ size }: ExtremeHaloProps): ReactElement {
  const box = useMemo(() => ({ width: size, height: size }), [size]);
  const ring = "absolute inset-0 rounded-full border-2 border-severity-extreme";
  return (
    <span aria-hidden="true" className="pointer-events-none relative block" style={box}>
      <span className={`${ring} animate-extreme-pulse motion-reduce:hidden`} />
      <span className={`${ring} hidden opacity-60 motion-reduce:block`} />
      <span className={`${ring} -inset-1.5 hidden opacity-30 motion-reduce:block`} />
    </span>
  );
}

export { ExtremeHalo };

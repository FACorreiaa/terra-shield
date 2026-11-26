import { useEffect, useState } from "react";

/** Ages ("2 min ago") are read again every 10 s, so they never freeze (the apps' `TimelineView`). */
const TICK_MS = 10_000;

function useTick(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(new Date());
    }, TICK_MS);
    return () => {
      clearInterval(timer);
    };
  }, []);
  return now;
}

export { useTick };

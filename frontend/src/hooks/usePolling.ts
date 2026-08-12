import { useEffect, useRef } from "react";

type PollingOptions = {
  enabled?: boolean;
};

/**
 * Runs `fn` immediately, then repeatedly every `intervalMs` (re-evaluated on each cycle if
 * a function, so callers can speed up/slow down based on the data they just fetched).
 * Pauses while the tab is hidden and fires an immediate refresh the moment it becomes
 * visible again, instead of quietly polling a backgrounded tab forever.
 */
export function usePolling(
  fn: () => void | Promise<void>,
  intervalMs: number | (() => number),
  options: PollingOptions = {},
) {
  const { enabled = true } = options;
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const intervalRef = useRef(intervalMs);
  intervalRef.current = intervalMs;

  useEffect(() => {
    if (!enabled) return;
    let timer: number | undefined;
    let cancelled = false;

    const resolveInterval = () => {
      const v = intervalRef.current;
      return typeof v === "function" ? v() : v;
    };

    const tick = () => {
      if (cancelled || document.hidden) return;
      void fnRef.current();
    };

    const schedule = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(() => {
        tick();
        schedule();
      }, resolveInterval());
    };

    tick();
    schedule();

    const onVisibility = () => {
      if (!document.hidden) tick();
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [enabled]);
}

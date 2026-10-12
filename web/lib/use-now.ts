'use client';

import { useEffect, useState } from 'react';

/**
 * Current time (ms) as render input, refreshed every `intervalMs` while
 * `active`. Calling Date.now() during render is impure (React Compiler); time
 * comes from here instead.
 */
export function useNow(active = true, intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [active, intervalMs]);
  return now;
}

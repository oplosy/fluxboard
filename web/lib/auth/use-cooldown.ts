'use client';

import { useCallback, useEffect, useState } from 'react';

/**
 * A seconds countdown for "resend" buttons, so users can't hammer the mailer
 * (the server throttles too) and can see when they may try again.
 */
export function useCooldown(): { remaining: number; start: (secs: number) => void } {
  const [until, setUntil] = useState(0);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (until <= now) return;
    const id = window.setInterval(() => setNow(Date.now()), 500);
    return () => window.clearInterval(id);
  }, [until, now]);

  const start = useCallback((secs: number) => {
    const t = Date.now();
    setNow(t);
    setUntil(t + secs * 1000);
  }, []);

  return { remaining: Math.max(0, Math.ceil((until - now) / 1000)), start };
}

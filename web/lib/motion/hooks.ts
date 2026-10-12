'use client';

import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore, type RefObject } from 'react';

const REDUCED_MOTION = '(prefers-reduced-motion: reduce)';

function subscribeReducedMotion(onChange: () => void): () => void {
  const mq = window.matchMedia(REDUCED_MOTION);
  mq.addEventListener('change', onChange);
  return () => mq.removeEventListener('change', onChange);
}

/** Live `prefers-reduced-motion` flag. SSR-safe (false on the server). */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribeReducedMotion, prefersReducedMotion, () => false);
}

export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia(REDUCED_MOTION).matches;
}

/**
 * Whether the element has scrolled into view (latches when `once`).
 * `observeParent` watches the parent box instead — needed when the element
 * itself is fully clipped (clip-path reveals never intersect).
 */
export function useInView<T extends Element>(
  ref: RefObject<T | null>,
  { once = true, rootMargin = '0px 0px -12% 0px', threshold = 0.1, observeParent = false } = {},
): boolean {
  const [inView, setInView] = useState(false);
  useEffect(() => {
    const el = observeParent ? ref.current?.parentElement : ref.current;
    if (!el) return;
    if (typeof IntersectionObserver === 'undefined') {
      // No observer to wait for: reveal on the next frame.
      const raf = requestAnimationFrame(() => setInView(true));
      return () => cancelAnimationFrame(raf);
    }
    const io = new IntersectionObserver(
      ([entry]) => {
        if (!entry) return;
        if (entry.isIntersecting) {
          setInView(true);
          if (once) io.disconnect();
        } else if (!once) {
          setInView(false);
        }
      },
      { rootMargin, threshold },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [ref, once, rootMargin, threshold, observeParent]);
  return inView;
}

/**
 * Mount/unmount with exit animations. While `open` flips to false the element
 * stays mounted for `duration` ms with `state === 'closed'`, so CSS can play
 * the exit before it leaves the DOM.
 */
export function usePresence(open: boolean, duration = 200): { mounted: boolean; state: 'open' | 'closed' } {
  // The last `open` value whose transition finished (entered after the extra
  // frame, or exited after `duration`). Only async callbacks write it; mounted
  // and state derive from it, so a change of `open` shows up in the same render.
  const [settled, setSettled] = useState(open);

  useEffect(() => {
    if (open) {
      // Next frame so the "closed" styles apply before transitioning to "open".
      let inner = 0;
      const outer = requestAnimationFrame(() => {
        inner = requestAnimationFrame(() => setSettled(true));
      });
      return () => {
        cancelAnimationFrame(outer);
        cancelAnimationFrame(inner);
      };
    }
    const t = window.setTimeout(() => setSettled(false), duration);
    return () => window.clearTimeout(t);
  }, [open, duration]);

  return { mounted: open || settled, state: open && settled ? 'open' : 'closed' };
}

/** Scroll progress (0..1) of an element travelling through the viewport. */
export function useScrollProgress<T extends HTMLElement>(ref: RefObject<T | null>): number {
  const [progress, setProgress] = useState(0);
  useEffect(() => {
    let raf = 0;
    const measure = () => {
      raf = 0;
      const el = ref.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const total = rect.height - window.innerHeight;
      const p = total <= 0 ? 0 : -rect.top / total;
      setProgress(Math.min(1, Math.max(0, p)));
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(measure);
    };
    measure();
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('resize', onScroll);
    return () => {
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [ref]);
  return progress;
}

/**
 * Keeps the latest value in a ref (for use inside long-lived callbacks). The ref
 * is updated after commit, never during render.
 */
export function useLatest<T>(value: T) {
  const ref = useRef(value);
  useLayoutEffect(() => {
    ref.current = value;
  });
  return ref;
}

'use client';

import { useEffect, useRef, useState, type RefObject } from 'react';

/** Live `prefers-reduced-motion` flag. SSR-safe (false until mounted). */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReduced(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, []);
  return reduced;
}

export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * Whether the element has scrolled into view (latches when `once`).
 * `observeParent` watches the parent box instead — needed when the element
 * itself is fully clipped (clip-path reveals never intersect).
 */
export function useInView<T extends Element>(
  ref: RefObject<T>,
  { once = true, rootMargin = '0px 0px -12% 0px', threshold = 0.1, observeParent = false } = {},
): boolean {
  const [inView, setInView] = useState(false);
  useEffect(() => {
    const el = observeParent ? ref.current?.parentElement : ref.current;
    if (!el) return;
    if (typeof IntersectionObserver === 'undefined') {
      setInView(true);
      return;
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
  const [mounted, setMounted] = useState(open);
  const [state, setState] = useState<'open' | 'closed'>(open ? 'open' : 'closed');

  useEffect(() => {
    if (open) {
      setMounted(true);
      // Next frame so the "closed" styles apply before transitioning to "open".
      const raf = requestAnimationFrame(() => requestAnimationFrame(() => setState('open')));
      return () => cancelAnimationFrame(raf);
    }
    setState('closed');
    const t = window.setTimeout(() => setMounted(false), duration);
    return () => window.clearTimeout(t);
  }, [open, duration]);

  return { mounted, state };
}

/** Scroll progress (0..1) of an element travelling through the viewport. */
export function useScrollProgress<T extends HTMLElement>(ref: RefObject<T>): number {
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

/** Keeps the latest value in a ref (for use inside long-lived callbacks). */
export function useLatest<T>(value: T) {
  const ref = useRef(value);
  ref.current = value;
  return ref;
}

'use client';

import { usePathname, useRouter } from 'next/navigation';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
  type RefObject,
} from 'react';

import { prefersReducedMotion } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';
import { LogoMark } from '@/components/brand/logo';

type Zone = 'public' | 'auth' | 'app' | 'admin';

function zoneOf(path: string): Zone {
  if (path === '/' || /^\/(features|pricing|changelog|status|legal|f)(\/|$)/.test(path)) return 'public';
  if (/^\/(login|register|forgot-password|reset-password|verify-email|invite|oauth|logout)(\/|$)/.test(path)) return 'auth';
  if (path.startsWith('/admin')) return 'admin';
  return 'app';
}

// The full-screen curtain is for "place" changes (marketing, auth, crossing
// into or out of the app). Inside the app, navigation stays instant and only a
// thin progress line runs, so working flows never wait on an animation.
function wantsCurtain(from: string, to: string): boolean {
  const a = zoneOf(from);
  const b = zoneOf(to);
  if (a !== b) return true;
  return a === 'public' || a === 'auth';
}

type Phase = 'idle' | 'enter' | 'covered' | 'reveal';

const BARS = 5;
const COVER_MS = 520;
const REVEAL_MS = 700;

interface TransitionNav {
  /** Navigate with the curtain (if the route change warrants one). */
  go: (href: string, opts?: { replace?: boolean }) => void;
}

const TransitionContext = createContext<TransitionNav | null>(null);

export function useTransitionNav(): TransitionNav {
  const ctx = useContext(TransitionContext);
  if (!ctx) throw new Error('useTransitionNav must be used within <RouteTransition>');
  return ctx;
}

export function RouteTransition({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [phase, setPhase] = useState<Phase>('idle');
  const [label, setLabel] = useState('');
  const [loading, setLoading] = useState<'off' | 'run' | 'done'>('off');
  const pending = useRef<{ href: string; replace: boolean } | null>(null);
  const fromPath = useRef(pathname);
  const timers = useRef<number[]>([]);

  const later = (fn: () => void, ms: number) => {
    timers.current.push(window.setTimeout(fn, ms));
  };

  const startCurtain = useCallback(
    (href: string, replace: boolean) => {
      const url = new URL(href, window.location.href);
      pending.current = { href: url.pathname + url.search + url.hash, replace };
      setLabel(url.pathname === '/' ? '/home' : url.pathname);
      setPhase('enter');
      requestAnimationFrame(() => requestAnimationFrame(() => setPhase('covered')));
      later(() => {
        const p = pending.current;
        if (!p) return;
        if (p.replace) router.replace(p.href);
        else router.push(p.href);
        // Safety net: reveal even if the pathname never changes (same route).
        later(() => setPhase((cur) => (cur === 'covered' ? 'reveal' : cur)), 4000);
      }, COVER_MS);
    },
    [router],
  );

  const go = useCallback(
    (href: string, opts?: { replace?: boolean }) => {
      const url = new URL(href, window.location.href);
      if (prefersReducedMotion() || !wantsCurtain(window.location.pathname, url.pathname)) {
        if (opts?.replace) router.replace(href);
        else router.push(href);
        return;
      }
      startCurtain(href, Boolean(opts?.replace));
    },
    [router, startCurtain],
  );

  // Intercept same-origin anchor clicks in the capture phase. Preventing the
  // default here makes next/link skip its own navigation (it checks
  // `defaultPrevented`), so the curtain can cover first.
  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const a = (e.target as Element | null)?.closest?.('a');
      if (!a || !a.href) return;
      if ((a.target && a.target !== '_self') || a.hasAttribute('download')) return;
      if (a.dataset.transition === 'none') return;
      const url = new URL(a.href, window.location.href);
      if (url.origin !== window.location.origin) return;
      if (url.pathname === window.location.pathname) return;

      if (!prefersReducedMotion() && wantsCurtain(window.location.pathname, url.pathname)) {
        e.preventDefault();
        startCurtain(url.href, false);
      } else {
        setLoading('run');
      }
    }
    document.addEventListener('click', onClick, true);
    return () => document.removeEventListener('click', onClick, true);
  }, [startCurtain]);

  // Route landed: lift the curtain / finish the progress line.
  useEffect(() => {
    if (pathname === fromPath.current) return;
    fromPath.current = pathname;
    setLoading((cur) => (cur === 'run' ? 'done' : cur));
    setPhase((cur) => {
      if (cur === 'covered' || cur === 'enter') {
        pending.current = null;
        return 'reveal';
      }
      return cur;
    });
  }, [pathname]);

  useEffect(() => {
    if (phase !== 'reveal') return;
    const t = window.setTimeout(() => setPhase('idle'), REVEAL_MS + BARS * 50);
    return () => window.clearTimeout(t);
  }, [phase]);

  useEffect(() => {
    if (loading === 'done') {
      const t = window.setTimeout(() => setLoading('off'), 450);
      return () => window.clearTimeout(t);
    }
    if (loading === 'run') {
      const t = window.setTimeout(() => setLoading('off'), 8000);
      return () => window.clearTimeout(t);
    }
  }, [loading]);

  useEffect(() => {
    const list = timers.current;
    return () => list.forEach((t) => window.clearTimeout(t));
  }, []);

  return (
    <TransitionContext.Provider value={{ go }}>
      {children}

      {/* In-zone progress line */}
      <div
        aria-hidden
        className={cn(
          'pointer-events-none fixed inset-x-0 top-0 z-[80] h-[2px] origin-left bg-signal',
          loading === 'off' && 'scale-x-0 opacity-0 transition-none',
          loading === 'run' && 'scale-x-[0.82] opacity-100 transition-transform duration-[2400ms] ease-out-expo',
          loading === 'done' && 'scale-x-100 opacity-0 transition-[transform,opacity] duration-400 ease-out-expo',
        )}
      />

      {/* Cross-place curtain: five columns sweep up, then lift away. */}
      {phase !== 'idle' ? (
        <div className="fixed inset-0 z-[90] flex" aria-hidden>
          {Array.from({ length: BARS }).map((_, i) => (
            <span
              key={i}
              className={cn(
                'h-full flex-1 bg-ink will-change-transform',
                i === BARS - 1 && 'bg-signal',
                phase === 'enter' && 'origin-bottom scale-y-0',
                phase === 'covered' && 'origin-bottom scale-y-100',
                phase === 'reveal' && 'origin-top scale-y-0',
              )}
              style={
                {
                  transitionProperty: 'transform',
                  transitionDuration: `${phase === 'reveal' ? REVEAL_MS : COVER_MS - 80}ms`,
                  transitionTimingFunction: 'var(--ease-in-out-quart)',
                  transitionDelay: `${(phase === 'reveal' ? i : BARS - 1 - i) * 45}ms`,
                } as CSSProperties
              }
            />
          ))}
          <div
            className={cn(
              'absolute inset-0 flex flex-col items-center justify-center gap-4 text-paper transition-opacity duration-200',
              phase === 'covered' ? 'opacity-100 delay-200' : 'opacity-0',
            )}
          >
            <LogoMark animated className="h-9 w-9" />
            <span className="font-mono text-[11px] uppercase tracking-[0.3em] text-paper/60">{label}</span>
          </div>
        </div>
      ) : null}
    </TransitionContext.Provider>
  );
}

/**
 * Plays a short enter animation on `ref` whenever the pathname changes —
 * without remounting the subtree (so board state, scroll and queries survive).
 * Paths matching `skip` (e.g. intercepted modal routes) don't animate.
 */
export function useRouteEnter(ref: RefObject<HTMLElement>, skip?: RegExp) {
  const pathname = usePathname();
  const prev = useRef(pathname);
  useEffect(() => {
    const before = prev.current;
    prev.current = pathname;
    if (before === pathname) return;
    if (skip && (skip.test(pathname) || skip.test(before))) return;
    const el = ref.current;
    if (!el || prefersReducedMotion() || typeof el.animate !== 'function') return;
    el.animate(
      [
        { opacity: 0, transform: 'translateY(10px)' },
        { opacity: 1, transform: 'none' },
      ],
      { duration: 520, easing: 'cubic-bezier(0.16, 1, 0.3, 1)' },
    );
  }, [pathname, ref, skip]);
}

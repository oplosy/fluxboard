'use client';

import {
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from 'react';

import { prefersReducedMotion, useInView } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

const GLYPHS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/<>_-+=#';

/**
 * Decodes text from random glyphs once in view (and again on hover when
 * `replayOnHover`). Width is held by an invisible copy so layout never jumps.
 */
export function ScrambleText({
  text,
  className,
  duration = 900,
  replayOnHover = false,
}: {
  text: string;
  className?: string;
  duration?: number;
  replayOnHover?: boolean;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref);
  const [shown, setShown] = useState(text);
  const raf = useRef(0);

  const play = () => {
    if (prefersReducedMotion()) {
      setShown(text);
      return;
    }
    cancelAnimationFrame(raf.current);
    const start = performance.now();
    const tick = (now: number) => {
      const p = Math.min(1, (now - start) / duration);
      const settled = Math.floor(p * text.length);
      let out = '';
      for (let i = 0; i < text.length; i++) {
        const ch = text[i] ?? '';
        if (i < settled || ch === ' ') out += ch;
        else out += GLYPHS[Math.floor(Math.random() * GLYPHS.length)];
      }
      setShown(out);
      if (p < 1) raf.current = requestAnimationFrame(tick);
    };
    raf.current = requestAnimationFrame(tick);
  };

  useEffect(() => {
    if (inView) play();
    return () => cancelAnimationFrame(raf.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inView, text]);

  return (
    <span
      ref={ref}
      className={cn('relative inline-block', className)}
      onPointerEnter={replayOnHover ? play : undefined}
      aria-label={text}
    >
      <span className="invisible" aria-hidden>
        {text}
      </span>
      <span className="absolute inset-0 whitespace-nowrap" aria-hidden>
        {shown}
      </span>
    </span>
  );
}

/** Counts from 0 to `value` with an expo-out curve once in view. */
export function CountUp({
  value,
  duration = 1400,
  decimals = 0,
  prefix = '',
  suffix = '',
  className,
}: {
  value: number;
  duration?: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
  className?: string;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref);
  const [n, setN] = useState(0);

  useEffect(() => {
    if (!inView) return;
    if (prefersReducedMotion()) {
      setN(value);
      return;
    }
    let raf = 0;
    const start = performance.now();
    const tick = (now: number) => {
      const p = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(2, -10 * p);
      setN(value * (p === 1 ? 1 : eased));
      if (p < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [inView, value, duration]);

  return (
    <span ref={ref} className={cn('tabular', className)}>
      {prefix}
      {n.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })}
      {suffix}
    </span>
  );
}

/** Pulls its child toward the pointer while hovered (spring back on leave). */
export function Magnetic({
  children,
  strength = 0.28,
  className,
}: {
  children: ReactNode;
  strength?: number;
  className?: string;
}) {
  const ref = useRef<HTMLSpanElement>(null);

  function onMove(e: ReactPointerEvent<HTMLSpanElement>) {
    const el = ref.current;
    if (!el || e.pointerType !== 'mouse' || prefersReducedMotion()) return;
    const r = el.getBoundingClientRect();
    const x = (e.clientX - (r.left + r.width / 2)) * strength;
    const y = (e.clientY - (r.top + r.height / 2)) * strength;
    el.style.transition = 'transform 0.2s var(--ease-out-expo)';
    el.style.transform = `translate3d(${x}px, ${y}px, 0)`;
  }

  function onLeave() {
    const el = ref.current;
    if (!el) return;
    el.style.transition = 'transform 0.7s var(--ease-spring)';
    el.style.transform = 'translate3d(0, 0, 0)';
  }

  return (
    <span ref={ref} className={cn('inline-block will-change-transform', className)} onPointerMove={onMove} onPointerLeave={onLeave}>
      {children}
    </span>
  );
}

/** 3D tilt toward the pointer with a moving specular highlight. */
export function Tilt({
  children,
  max = 7,
  className,
  style,
}: {
  children: ReactNode;
  max?: number;
  className?: string;
  style?: CSSProperties;
}) {
  const ref = useRef<HTMLDivElement>(null);

  function onMove(e: ReactPointerEvent<HTMLDivElement>) {
    const el = ref.current;
    if (!el || e.pointerType !== 'mouse' || prefersReducedMotion()) return;
    const r = el.getBoundingClientRect();
    const px = (e.clientX - r.left) / r.width;
    const py = (e.clientY - r.top) / r.height;
    el.style.transition = 'transform 0.15s linear';
    el.style.transform = `perspective(900px) rotateX(${(0.5 - py) * max}deg) rotateY(${(px - 0.5) * max}deg) translateZ(0)`;
    el.style.setProperty('--mx', `${px * 100}%`);
    el.style.setProperty('--my', `${py * 100}%`);
  }

  function onLeave() {
    const el = ref.current;
    if (!el) return;
    el.style.transition = 'transform 0.8s var(--ease-spring)';
    el.style.transform = 'perspective(900px) rotateX(0deg) rotateY(0deg)';
  }

  return (
    <div
      ref={ref}
      className={cn('group/tilt relative [transform-style:preserve-3d]', className)}
      style={style}
      onPointerMove={onMove}
      onPointerLeave={onLeave}
    >
      {children}
      <span
        aria-hidden
        className="pointer-events-none absolute inset-0 rounded-[inherit] opacity-0 transition-opacity duration-300 group-hover/tilt:opacity-100"
        style={{
          background:
            'radial-gradient(420px circle at var(--mx, 50%) var(--my, 50%), hsl(var(--foreground) / 0.06), transparent 45%)',
        }}
      />
    </div>
  );
}

/** Infinite horizontal ticker; pauses on hover. Children are rendered twice. */
export function Marquee({
  children,
  duration = 40,
  reverse = false,
  className,
}: {
  children: ReactNode;
  duration?: number;
  reverse?: boolean;
  className?: string;
}) {
  return (
    <div className={cn('group/marquee mask-fade-x flex overflow-hidden', className)}>
      <div
        className="flex shrink-0 animate-marquee items-center group-hover/marquee:[animation-play-state:paused]"
        style={
          {
            '--marquee-duration': `${duration}s`,
            animationDirection: reverse ? 'reverse' : 'normal',
          } as CSSProperties
        }
      >
        <div className="flex shrink-0 items-center">{children}</div>
        <div className="flex shrink-0 items-center" aria-hidden>
          {children}
        </div>
      </div>
    </div>
  );
}

/** Thin signal-coloured bar tracking page scroll. */
export function ScrollProgress() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let raf = 0;
    const update = () => {
      raf = 0;
      const el = ref.current;
      if (!el) return;
      const max = document.documentElement.scrollHeight - window.innerHeight;
      el.style.transform = `scaleX(${max > 0 ? window.scrollY / max : 0})`;
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(update);
    };
    update();
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('resize', onScroll);
    return () => {
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);
  return (
    <div
      ref={ref}
      aria-hidden
      className="pointer-events-none fixed inset-x-0 top-0 z-[55] h-[2px] origin-left scale-x-0 bg-signal"
    />
  );
}

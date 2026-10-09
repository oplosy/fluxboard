'use client';

import { useEffect, type RefObject } from 'react';

/** Raw HSL triplets ("14 88% 51%") read from the CSS design tokens. */
export interface Palette {
  fg: string;
  bg: string;
  card: string;
  muted: string;
  border: string;
  signal: string;
  cobalt: string;
  ink: string;
  paper: string;
  success: string;
  destructive: string;
  warning: string;
  dark: boolean;
}

/** Build a canvas colour string from a token triplet. */
export function hsl(triplet: string, alpha = 1): string {
  return alpha >= 1 ? `hsl(${triplet})` : `hsl(${triplet} / ${alpha})`;
}

export function readPalette(): Palette {
  const s = getComputedStyle(document.documentElement);
  const v = (name: string) => s.getPropertyValue(name).trim() || '0 0% 50%';
  return {
    fg: v('--foreground'),
    bg: v('--background'),
    card: v('--card'),
    muted: v('--muted-foreground'),
    border: v('--border'),
    signal: v('--signal'),
    cobalt: v('--cobalt'),
    ink: v('--ink'),
    paper: v('--paper'),
    success: v('--success'),
    destructive: v('--destructive'),
    warning: v('--warning'),
    dark: document.documentElement.classList.contains('dark'),
  };
}

export interface Scene {
  /** Called on mount and whenever the canvas' CSS size changes. */
  resize(width: number, height: number): void;
  /** Draw one frame. `t` = ms since start, `dt` = ms since last frame (clamped). */
  frame(ctx: CanvasRenderingContext2D, t: number, dt: number): void;
  palette?(p: Palette): void;
  /** Pointer position in canvas CSS pixels; `inside` is false when it leaves. */
  pointer?(x: number, y: number, inside: boolean): void;
  press?(x: number, y: number): void;
}

/**
 * Runs a canvas `Scene`: DPR-aware sizing, a rAF loop that pauses while the
 * canvas is off-screen or the tab is hidden, theme-change palette updates,
 * and pointer forwarding. With reduced motion the scene is advanced a fixed
 * number of frames once and then frozen.
 */
export function useScene(
  ref: RefObject<HTMLCanvasElement>,
  create: () => Scene,
  { pointerTarget = 'canvas', deps = [] as unknown[] }: { pointerTarget?: 'canvas' | 'window'; deps?: unknown[] } = {},
) {
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const scene = create();
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    let width = 0;
    let height = 0;
    let raf = 0;
    let visible = true;
    let last = 0;
    let t = 0;

    scene.palette?.(readPalette());

    const resize = () => {
      const rect = canvas.getBoundingClientRect();
      width = Math.max(1, Math.round(rect.width));
      height = Math.max(1, Math.round(rect.height));
      canvas.width = Math.round(width * dpr);
      canvas.height = Math.round(height * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      scene.resize(width, height);
      if (reduced) still();
    };

    const still = () => {
      for (let i = 0; i < 120; i++) {
        t += 16;
        scene.frame(ctx, t, 16);
      }
    };

    const loop = (now: number) => {
      raf = 0;
      if (!visible || document.hidden) return;
      const dt = last ? Math.min(48, now - last) : 16;
      last = now;
      t += dt;
      scene.frame(ctx, t, dt);
      raf = requestAnimationFrame(loop);
    };

    const start = () => {
      if (reduced || raf || !visible || document.hidden) return;
      last = 0;
      raf = requestAnimationFrame(loop);
    };

    const ro = new ResizeObserver(resize);
    ro.observe(canvas);
    resize();

    const io = new IntersectionObserver(([entry]) => {
      visible = Boolean(entry?.isIntersecting);
      if (visible) start();
    });
    io.observe(canvas);

    const onVisibility = () => {
      if (!document.hidden) start();
    };
    document.addEventListener('visibilitychange', onVisibility);

    const mo = new MutationObserver(() => {
      scene.palette?.(readPalette());
      if (reduced) still();
    });
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

    const target: HTMLElement | Window = pointerTarget === 'window' ? window : canvas;
    const local = (e: PointerEvent) => {
      const r = canvas.getBoundingClientRect();
      return { x: e.clientX - r.left, y: e.clientY - r.top, r };
    };
    const onMove = (e: Event) => {
      const { x, y } = local(e as PointerEvent);
      scene.pointer?.(x, y, x >= 0 && y >= 0 && x <= width && y <= height);
    };
    const onLeave = () => scene.pointer?.(-9999, -9999, false);
    const onDown = (e: Event) => {
      const { x, y } = local(e as PointerEvent);
      if (x >= 0 && y >= 0 && x <= width && y <= height) scene.press?.(x, y);
    };
    target.addEventListener('pointermove', onMove, { passive: true });
    target.addEventListener('pointerdown', onDown, { passive: true });
    canvas.addEventListener('pointerleave', onLeave);

    start();
    if (reduced) still();

    return () => {
      if (raf) cancelAnimationFrame(raf);
      ro.disconnect();
      io.disconnect();
      mo.disconnect();
      document.removeEventListener('visibilitychange', onVisibility);
      target.removeEventListener('pointermove', onMove);
      target.removeEventListener('pointerdown', onDown);
      canvas.removeEventListener('pointerleave', onLeave);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
}

// ── Small math helpers shared by scenes ────────────────────────────────

/** Deterministic PRNG (mulberry32) so scenes look the same on every load. */
export function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let x = a;
    x = Math.imul(x ^ (x >>> 15), x | 1);
    x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

/** 3D value noise in [-1, 1]; cheap and smooth enough for flow fields. */
export function createNoise(seed = 7) {
  const rand = rng(seed);
  const perm = new Uint8Array(512);
  const p = Array.from({ length: 256 }, (_, i) => i);
  for (let i = 255; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1));
    [p[i], p[j]] = [p[j] as number, p[i] as number];
  }
  for (let i = 0; i < 512; i++) perm[i] = p[i & 255] as number;
  const vals = new Float32Array(256);
  for (let i = 0; i < 256; i++) vals[i] = rand() * 2 - 1;

  const fade = (t: number) => t * t * (3 - 2 * t);
  const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
  const h = (x: number, y: number, z: number) =>
    vals[perm[(perm[(perm[x & 255] as number) + (y & 255)] as number) + (z & 255)] as number] as number;

  return (x: number, y: number, z: number) => {
    const xi = Math.floor(x);
    const yi = Math.floor(y);
    const zi = Math.floor(z);
    const xf = fade(x - xi);
    const yf = fade(y - yi);
    const zf = fade(z - zi);
    const x1 = lerp(h(xi, yi, zi), h(xi + 1, yi, zi), xf);
    const x2 = lerp(h(xi, yi + 1, zi), h(xi + 1, yi + 1, zi), xf);
    const x3 = lerp(h(xi, yi, zi + 1), h(xi + 1, yi, zi + 1), xf);
    const x4 = lerp(h(xi, yi + 1, zi + 1), h(xi + 1, yi + 1, zi + 1), xf);
    return lerp(lerp(x1, x2, yf), lerp(x3, x4, yf), zf);
  };
}

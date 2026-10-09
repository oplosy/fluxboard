'use client';

import { useEffect, useRef } from 'react';

import { hsl, readPalette, type Palette } from './scene';

interface Piece {
  x: number;
  y: number;
  vx: number;
  vy: number;
  rot: number;
  vr: number;
  w: number;
  h: number;
  color: string;
  life: number;
  max: number;
}

type BurstListener = (x: number, y: number, power: number) => void;
const listeners = new Set<BurstListener>();

/**
 * Fire a confetti-style burst at viewport coordinates. Used for "done"
 * moments (a card landing in the last column, a plan upgrade). No-op with
 * reduced motion or before the layer mounts.
 */
export function burst(x: number, y: number, power = 1) {
  if (typeof window === 'undefined') return;
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  listeners.forEach((fn) => fn(x, y, power));
}

/** Fire a burst from the centre of an element. */
export function burstFrom(el: Element | null, power = 1) {
  if (!el) return;
  const r = el.getBoundingClientRect();
  burst(r.left + r.width / 2, r.top + r.height / 2, power);
}

/**
 * Full-viewport, pointer-transparent canvas that only runs a rAF loop while
 * pieces are alive.
 */
export function BurstLayer() {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx) return;
    let pieces: Piece[] = [];
    let raf = 0;
    let last = 0;
    let pal: Palette = readPalette();
    const dpr = Math.min(window.devicePixelRatio || 1, 2);

    const size = () => {
      canvas.width = Math.round(window.innerWidth * dpr);
      canvas.height = Math.round(window.innerHeight * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    size();
    window.addEventListener('resize', size);

    const loop = (now: number) => {
      const dt = last ? Math.min(40, now - last) : 16;
      last = now;
      const k = dt / 16;
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
      pieces = pieces.filter((p) => (p.life += dt) < p.max);
      for (const p of pieces) {
        p.vy += 0.32 * k;
        p.vx *= Math.pow(0.985, k);
        p.vy *= Math.pow(0.985, k);
        p.x += p.vx * k;
        p.y += p.vy * k;
        p.rot += p.vr * k;
        const fade = 1 - Math.max(0, (p.life - p.max * 0.6) / (p.max * 0.4));
        ctx.save();
        ctx.translate(p.x, p.y);
        ctx.rotate(p.rot);
        // Flip the piece's width with its rotation to fake a 3D tumble.
        ctx.scale(Math.cos(p.rot * 2.3), 1);
        ctx.globalAlpha = fade;
        ctx.fillStyle = p.color;
        ctx.fillRect(-p.w / 2, -p.h / 2, p.w, p.h);
        ctx.restore();
      }
      if (pieces.length) raf = requestAnimationFrame(loop);
      else {
        raf = 0;
        last = 0;
        ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
      }
    };

    const onBurst: BurstListener = (x, y, power) => {
      pal = readPalette();
      const colors = [hsl(pal.signal), hsl(pal.signal), hsl(pal.fg), hsl(pal.cobalt), hsl(pal.success)];
      const n = Math.round(42 * power);
      for (let i = 0; i < n; i++) {
        const a = -Math.PI / 2 + (Math.random() - 0.5) * Math.PI * 1.25;
        const v = (5 + Math.random() * 8) * Math.sqrt(power);
        pieces.push({
          x,
          y,
          vx: Math.cos(a) * v,
          vy: Math.sin(a) * v,
          rot: Math.random() * Math.PI,
          vr: (Math.random() - 0.5) * 0.4,
          w: 4 + Math.random() * 5,
          h: 2 + Math.random() * 3,
          color: colors[i % colors.length] as string,
          life: 0,
          max: 1100 + Math.random() * 700,
        });
      }
      if (!raf) raf = requestAnimationFrame(loop);
    };
    listeners.add(onBurst);

    return () => {
      listeners.delete(onBurst);
      window.removeEventListener('resize', size);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  return <canvas ref={ref} aria-hidden className="pointer-events-none fixed inset-0 z-[100] h-screen w-screen" />;
}

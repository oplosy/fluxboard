'use client';

import { useRef } from 'react';

import { cn } from '@/lib/utils';
import { hsl, rng, useScene, type Palette, type Scene } from './scene';

interface Dot {
  hx: number;
  hy: number;
  x: number;
  y: number;
  vx: number;
  vy: number;
  hot: boolean;
}

/**
 * Text sampled into particles that spring to their home positions. The
 * pointer pushes them away; a press detonates them outward. Used for 404.
 */
function createScatterScene(text: string, getFont: () => string): Scene {
  const rand = rng(404);
  let W = 1;
  let H = 1;
  let dots: Dot[] = [];
  let pal: Palette | null = null;
  const mouse = { x: -9999, y: -9999 };

  const sample = () => {
    const off = document.createElement('canvas');
    off.width = W;
    off.height = H;
    const o = off.getContext('2d', { willReadFrequently: true });
    if (!o) return;
    let size = Math.min(H * 0.9, W * 0.42);
    o.font = `800 ${size}px ${getFont()}`;
    while (o.measureText(text).width > W * 0.92 && size > 20) {
      size -= 4;
      o.font = `800 ${size}px ${getFont()}`;
    }
    o.textAlign = 'center';
    o.textBaseline = 'middle';
    o.fillStyle = '#000';
    o.fillText(text, W / 2, H / 2);
    const data = o.getImageData(0, 0, W, H).data;
    const step = Math.max(4, Math.round(size / 42));
    const next: Dot[] = [];
    for (let y = 0; y < H; y += step) {
      for (let x = 0; x < W; x += step) {
        if ((data[(y * W + x) * 4 + 3] as number) > 128) {
          const old = dots[next.length];
          next.push({
            hx: x,
            hy: y,
            x: old?.x ?? rand() * W,
            y: old?.y ?? rand() * H,
            vx: 0,
            vy: 0,
            hot: rand() < 0.06,
          });
        }
      }
    }
    dots = next;
  };

  return {
    resize(w, h) {
      const first = W === 1;
      W = w;
      H = h;
      sample();
      if (first && document.fonts?.ready) void document.fonts.ready.then(sample);
    },
    palette(p) {
      pal = p;
    },
    pointer(x, y) {
      mouse.x = x;
      mouse.y = y;
    },
    press(x, y) {
      for (const d of dots) {
        const dx = d.x - x;
        const dy = d.y - y;
        const dist = Math.hypot(dx, dy) || 1;
        const f = Math.min(30, 2400 / dist);
        d.vx += (dx / dist) * f;
        d.vy += (dy / dist) * f;
      }
    },
    frame(ctx, _t, dt) {
      if (!pal) return;
      const k = dt / 16;
      ctx.clearRect(0, 0, W, H);
      const ink = hsl(pal.fg, 0.85);
      const sig = hsl(pal.signal);
      for (const d of dots) {
        const dx = d.x - mouse.x;
        const dy = d.y - mouse.y;
        const dist2 = dx * dx + dy * dy;
        if (dist2 < 90 * 90) {
          const dist = Math.sqrt(dist2) || 1;
          const f = (1 - dist / 90) * 3.5;
          d.vx += (dx / dist) * f;
          d.vy += (dy / dist) * f;
        }
        d.vx += (d.hx - d.x) * 0.045 * k;
        d.vy += (d.hy - d.y) * 0.045 * k;
        d.vx *= Math.pow(0.84, k);
        d.vy *= Math.pow(0.84, k);
        d.x += d.vx * k;
        d.y += d.vy * k;
        ctx.fillStyle = d.hot ? sig : ink;
        ctx.fillRect(d.x - 1.5, d.y - 1.5, 3, 3);
      }
    },
  };
}

export function ScatterText({ text, className }: { text: string; className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useScene(
    ref,
    () =>
      createScatterScene(text, () => {
        const v = getComputedStyle(document.documentElement).getPropertyValue('--font-display').trim();
        return v || 'sans-serif';
      }),
    { pointerTarget: 'window', deps: [text] },
  );
  return <canvas ref={ref} className={cn('block h-full w-full', className)} aria-hidden />;
}

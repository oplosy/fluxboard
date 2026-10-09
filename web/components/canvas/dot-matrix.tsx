'use client';

import { useRef } from 'react';

import { cn } from '@/lib/utils';
import { hsl, useScene, type Palette, type Scene } from './scene';

interface Ripple {
  x: number;
  y: number;
  age: number;
}

/**
 * LED dot-matrix panel. Words are rasterised into the grid and cross-fade
 * with a left-to-right wipe; an idle wave breathes through the field, the
 * pointer lifts nearby dots and a press sends a ripple across the board.
 * Always rendered on an ink background, so it reads the same in both themes.
 */
function createMatrixScene(words: string[], getFont: () => string): Scene {
  const GAP = 13;
  let W = 1;
  let H = 1;
  let cols = 0;
  let rows = 0;
  let pal: Palette | null = null;
  let masks: Float32Array[] = [];
  let level = new Float32Array(0);
  const ripples: Ripple[] = [];
  const mouse = { x: -9999, y: -9999 };
  const HOLD = 3400;

  const rasterise = () => {
    cols = Math.ceil(W / GAP) + 1;
    rows = Math.ceil(H / GAP) + 1;
    level = new Float32Array(cols * rows);
    const off = document.createElement('canvas');
    off.width = cols;
    off.height = rows;
    const o = off.getContext('2d', { willReadFrequently: true });
    if (!o) return;
    masks = words.map((word) => {
      o.clearRect(0, 0, cols, rows);
      o.fillStyle = '#fff';
      o.textAlign = 'center';
      o.textBaseline = 'middle';
      let size = Math.floor(rows * 0.5);
      o.font = `700 ${size}px ${getFont()}`;
      while (o.measureText(word).width > cols * 0.86 && size > 6) {
        size -= 1;
        o.font = `700 ${size}px ${getFont()}`;
      }
      o.fillText(word, cols / 2, rows * 0.46);
      const data = o.getImageData(0, 0, cols, rows).data;
      const m = new Float32Array(cols * rows);
      for (let i = 0; i < m.length; i++) m[i] = (data[i * 4 + 3] as number) / 255;
      return m;
    });
  };

  return {
    resize(w, h) {
      const first = W === 1;
      W = w;
      H = h;
      rasterise();
      // Re-rasterise once the display font has loaded (first pass may use the fallback).
      if (first && document.fonts?.ready) void document.fonts.ready.then(rasterise);
    },
    palette(p) {
      pal = p;
    },
    pointer(x, y) {
      mouse.x = x;
      mouse.y = y;
    },
    press(x, y) {
      ripples.push({ x, y, age: 0 });
    },
    frame(ctx, t, dt) {
      if (!pal || masks.length === 0) return;
      ctx.clearRect(0, 0, W, H);

      const cycle = Math.floor(t / HOLD);
      const cur = masks[cycle % masks.length] as Float32Array;
      const prev = masks[(cycle + masks.length - 1) % masks.length] as Float32Array;
      const into = (t % HOLD) / 900; // wipe progress, >1 means settled

      for (let i = ripples.length - 1; i >= 0; i--) {
        const r = ripples[i] as Ripple;
        r.age += dt;
        if (r.age > 2200) ripples.splice(i, 1);
      }

      for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
          const idx = r * cols + c;
          const x = c * GAP;
          const y = r * GAP;
          const wipe = into * 1.4 - c / cols;
          const target = wipe > 0 ? (cur[idx] as number) : (prev[idx] as number);
          // Ease each dot toward its target for a soft LED response.
          const lv = (level[idx] as number) + (target - (level[idx] as number)) * Math.min(1, dt / 90);
          level[idx] = lv;

          const wave = (Math.sin(x * 0.012 + y * 0.018 - t * 0.0016) + 1) * 0.5;
          let boost = 0;
          const dx = x - mouse.x;
          const dy = y - mouse.y;
          const d = Math.sqrt(dx * dx + dy * dy);
          if (d < 110) boost += (1 - d / 110) * 0.9;
          for (const rp of ripples) {
            const rr = (rp.age / 1000) * 420;
            const dd = Math.abs(Math.hypot(x - rp.x, y - rp.y) - rr);
            if (dd < 26) boost += (1 - dd / 26) * (1 - rp.age / 2200) * 1.2;
          }

          const base = 0.08 + wave * 0.08;
          const radius = 1.2 + lv * 4 + boost * 1.8;
          if (lv > 0.08) {
            ctx.fillStyle = hsl(pal.signal, Math.min(1, 0.55 + lv * 0.45 + boost * 0.3));
          } else {
            ctx.fillStyle = `hsl(42 23% 95% / ${Math.min(0.9, base + boost * 0.55)})`;
          }
          ctx.fillRect(x - radius / 2, y - radius / 2, radius, radius);
        }
      }
    },
  };
}

export function DotMatrix({ words, className }: { words: string[]; className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useScene(
    ref,
    () =>
      createMatrixScene(words, () => {
        const v = getComputedStyle(document.documentElement).getPropertyValue('--font-display').trim();
        return v || 'sans-serif';
      }),
    { deps: [words.join('|')] },
  );
  return <canvas ref={ref} className={cn('block h-full w-full', className)} aria-hidden />;
}

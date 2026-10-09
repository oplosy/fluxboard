'use client';

import { useRef } from 'react';

import { cn } from '@/lib/utils';
import { createNoise, hsl, rng, useScene, type Palette, type Scene } from './scene';

/** Lane boundaries as fractions of the canvas width (shared with the DOM labels). */
export const FLUX_LANES = [0.36, 0.52, 0.68, 0.84, 1] as const;

interface Particle {
  x: number;
  y: number;
  px: number;
  py: number;
  speed: number;
  lane: number;
}

interface Task {
  x: number;
  y: number;
  speed: number;
  lane: number;
  w: number;
}

interface Ring {
  x: number;
  y: number;
  age: number;
}

function laneOf(x: number, width: number): number {
  const f = x / width;
  for (let i = 0; i < FLUX_LANES.length; i++) {
    if (f < (FLUX_LANES[i] as number)) return i;
  }
  return FLUX_LANES.length - 1;
}

/**
 * Hero flow field. Thousands of particles ride a noise field left → right
 * across four lanes (backlog → done); crossing into the last lane turns them
 * signal-coloured. Larger "task" glyphs travel slower and fire a pulse when
 * they ship. The pointer stirs the field into a vortex; pressing scatters a
 * burst of new particles.
 */
function createFluxScene(onShip: () => void): Scene {
  const noise = createNoise(11);
  const rand = rng(42);
  let W = 1;
  let H = 1;
  let pal: Palette | null = null;
  let particles: Particle[] = [];
  let tasks: Task[] = [];
  const rings: Ring[] = [];
  const mouse = { x: -9999, y: -9999, on: false };

  const spawnX = () => rand() * W * 0.34;

  const seed = () => {
    const count = Math.min(1700, Math.round((W * H) / 650));
    particles = Array.from({ length: count }, () => {
      const x = rand() * W;
      const y = rand() * H;
      return { x, y, px: x, py: y, speed: 0.6 + rand() * 0.9, lane: laneOf(x, W) };
    });
    tasks = Array.from({ length: 9 }, (_, i) => ({
      x: W * (0.05 + (i / 9) * 0.9),
      y: H * (0.15 + rand() * 0.7),
      speed: 0.22 + rand() * 0.18,
      lane: 0,
      w: 12 + rand() * 8,
    }));
    tasks.forEach((t) => (t.lane = laneOf(t.x, W)));
  };

  const field = (x: number, y: number, t: number) => {
    const a = noise(x * 0.0017, y * 0.0024, t * 0.00006) * 2.4;
    let vx = 0.85 + Math.cos(a) * 0.75;
    let vy = Math.sin(a) * 0.95;
    if (mouse.on) {
      const dx = x - mouse.x;
      const dy = y - mouse.y;
      const d2 = dx * dx + dy * dy;
      const R = 150;
      if (d2 < R * R) {
        const d = Math.sqrt(d2) || 1;
        const f = (1 - d / R) * 3.2;
        vx += (-dy / d) * f + (dx / d) * f * 0.35;
        vy += (dx / d) * f + (dy / d) * f * 0.35;
      }
    }
    return { vx, vy };
  };

  return {
    resize(w, h) {
      const first = W === 1;
      W = w;
      H = h;
      if (first || particles.length === 0) seed();
    },
    palette(p) {
      pal = p;
    },
    pointer(x, y, inside) {
      mouse.x = x;
      mouse.y = y;
      mouse.on = inside;
    },
    press(x, y) {
      for (let i = 0; i < 60; i++) {
        const a = rand() * Math.PI * 2;
        const r = rand() * 26;
        const px = x + Math.cos(a) * r;
        const py = y + Math.sin(a) * r;
        particles.push({ x: px, y: py, px, py, speed: 1.4 + rand() * 1.2, lane: laneOf(px, W) });
      }
      if (particles.length > 2600) particles.splice(0, particles.length - 2600);
    },
    frame(ctx, t, dt) {
      if (!pal) return;
      const k = dt / 16;

      // Fade previous frame toward transparent so the page shows through.
      ctx.globalCompositeOperation = 'destination-out';
      ctx.fillStyle = 'rgba(0,0,0,0.11)';
      ctx.fillRect(0, 0, W, H);
      ctx.globalCompositeOperation = 'source-over';

      const lanes = FLUX_LANES.length;
      const paths: Path2D[] = Array.from({ length: lanes }, () => new Path2D());

      for (const p of particles) {
        const { vx, vy } = field(p.x, p.y, t);
        p.px = p.x;
        p.py = p.y;
        p.x += vx * p.speed * k * 1.4;
        p.y += vy * p.speed * k * 1.4;
        if (p.x > W + 4 || p.y < -8 || p.y > H + 8) {
          p.x = p.px = spawnX();
          p.y = p.py = rand() * H;
          p.lane = laneOf(p.x, W);
          continue;
        }
        p.lane = laneOf(p.x, W);
        const path = paths[p.lane] as Path2D;
        path.moveTo(p.px, p.py);
        path.lineTo(p.x, p.y);
      }

      ctx.lineWidth = 1.1;
      ctx.lineCap = 'round';
      for (let i = 0; i < lanes; i++) {
        const isDone = i === lanes - 1;
        ctx.strokeStyle = isDone ? hsl(pal.signal, 0.85) : hsl(pal.fg, 0.16 + i * 0.09);
        ctx.stroke(paths[i] as Path2D);
      }

      // Task glyphs: slower, crisp, ship with a pulse.
      for (const task of tasks) {
        const { vx, vy } = field(task.x, task.y, t);
        task.x += vx * task.speed * k * 1.4;
        task.y += vy * task.speed * k * 0.6;
        task.y = Math.min(H - 20, Math.max(20, task.y));
        const lane = laneOf(task.x, W);
        if (lane !== task.lane) {
          task.lane = lane;
          if (lane === lanes - 1) {
            rings.push({ x: task.x, y: task.y, age: 0 });
            onShip();
          }
        }
        if (task.x > W + 30) {
          task.x = spawnX();
          task.y = H * (0.12 + rand() * 0.76);
          task.lane = laneOf(task.x, W);
        }
        const done = task.lane === lanes - 1;
        ctx.fillStyle = done ? hsl(pal.signal) : hsl(pal.fg, 0.9);
        ctx.fillRect(task.x - task.w / 2, task.y - 3.5, task.w, 7);
      }

      for (let i = rings.length - 1; i >= 0; i--) {
        const r = rings[i] as Ring;
        r.age += dt;
        const p = r.age / 900;
        if (p >= 1) {
          rings.splice(i, 1);
          continue;
        }
        ctx.beginPath();
        ctx.arc(r.x, r.y, 6 + p * 46, 0, Math.PI * 2);
        ctx.strokeStyle = hsl(pal.signal, (1 - p) * 0.8);
        ctx.lineWidth = 1.5;
        ctx.stroke();
      }
    },
  };
}

export function FluxField({ className, onShip }: { className?: string; onShip?: () => void }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const shipRef = useRef(onShip);
  shipRef.current = onShip;
  useScene(ref, () => createFluxScene(() => shipRef.current?.()), { pointerTarget: 'window' });
  return <canvas ref={ref} className={cn('block h-full w-full', className)} aria-hidden />;
}

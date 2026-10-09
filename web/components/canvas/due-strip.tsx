'use client';

import { useRef, useState } from 'react';

import { cn } from '@/lib/utils';
import { hsl, useScene, type Palette, type Scene } from './scene';

export interface DueItem {
  id: string;
  title: string;
  due: string; // ISO date
  priority: string;
}

interface Placed extends DueItem {
  day: number; // days from today (can be negative)
  stack: number;
  x: number;
  y: number;
  born: number;
}

const DAY = 86_400_000;
const PAST = 4;
const FUTURE = 14;

function startOfDay(d: Date) {
  const c = new Date(d);
  c.setHours(0, 0, 0, 0);
  return c.getTime();
}

/**
 * Two-and-a-half-week due-date strip for the current user's tasks. Tasks drop
 * onto their due day and stack; the "now" cursor creeps along today's column;
 * overdue work pulses in the destructive colour. Hover reports the nearest
 * task to the parent for an HTML tooltip.
 */
function createDueScene(
  items: DueItem[],
  getMono: () => string,
  onHover: (hit: { title: string; x: number; y: number; due: string } | null) => void,
): Scene {
  let W = 1;
  let H = 1;
  let pal: Palette | null = null;
  let placed: Placed[] = [];
  let hovered: string | null = null;
  const today = startOfDay(new Date());

  const left = 16;
  const right = 16;
  const base = () => H - 30;
  const xOf = (day: number) => left + ((day + PAST) / (PAST + FUTURE)) * (W - left - right);

  const layout = () => {
    const perDay = new Map<number, number>();
    placed = items
      .map((it) => ({ it, day: Math.round((startOfDay(new Date(it.due)) - today) / DAY) }))
      .filter(({ day }) => day >= -PAST && day <= FUTURE)
      .sort((a, b) => a.day - b.day)
      .map(({ it, day }, i) => {
        const stack = perDay.get(day) ?? 0;
        perDay.set(day, stack + 1);
        const colW = (W - left - right) / (PAST + FUTURE);
        return {
          ...it,
          day,
          stack,
          x: xOf(day) + colW / 2,
          y: base() - 14 - stack * 15,
          born: 250 + i * 70,
        };
      });
  };

  return {
    resize(w, h) {
      W = w;
      H = h;
      layout();
    },
    palette(p) {
      pal = p;
    },
    pointer(x, y, inside) {
      let best: Placed | null = null;
      let bestD = 14;
      if (inside) {
        for (const p of placed) {
          const d = Math.hypot(p.x - x, p.y - y);
          if (d < bestD) {
            best = p;
            bestD = d;
          }
        }
      }
      const id = best?.id ?? null;
      if (id !== hovered) {
        hovered = id;
        onHover(best ? { title: best.title, x: best.x, y: best.y, due: best.due } : null);
      }
    },
    frame(ctx, t) {
      if (!pal) return;
      ctx.clearRect(0, 0, W, H);
      const mono = getMono();
      const span = PAST + FUTURE;
      const colW = (W - left - right) / span;
      const intro = Math.min(1, t / 900);
      const ease = 1 - Math.pow(1 - intro, 3);

      // Day grid + labels, drawn in from the left.
      ctx.font = `10px ${mono}`;
      ctx.textAlign = 'center';
      for (let d = -PAST; d <= FUTURE; d++) {
        const x = xOf(d);
        if (x > left + (W - left - right) * ease + 1) break;
        const date = new Date(today + d * DAY);
        const weekend = date.getDay() === 0 || date.getDay() === 6;
        if (weekend && d < FUTURE) {
          ctx.fillStyle = hsl(pal.fg, 0.035);
          ctx.fillRect(x, 8, colW, base() - 8);
        }
        ctx.fillStyle = hsl(pal.fg, d === 0 ? 0.5 : 0.14);
        ctx.fillRect(x, base(), 1, 6);
        if (d < FUTURE) {
          ctx.fillStyle = hsl(pal.fg, d === 0 ? 0.9 : 0.42);
          const label = d === 0 ? 'TODAY' : date.getDate() === 1 || d === -PAST ? date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }).toUpperCase() : String(date.getDate());
          ctx.fillText(label, x + colW / 2, base() + 20);
        }
      }
      ctx.fillStyle = hsl(pal.fg, 0.25);
      ctx.fillRect(left, base(), (W - left - right) * ease, 1);

      // "Now" cursor travels across today's column through the day.
      const now = new Date();
      const frac = (now.getHours() * 60 + now.getMinutes()) / 1440;
      const nx = xOf(0) + colW * frac;
      if (intro >= 1) {
        ctx.fillStyle = hsl(pal.signal, 0.9);
        ctx.fillRect(nx, 6, 1.5, base() - 6);
        const blink = (Math.sin(t / 380) + 1) / 2;
        ctx.beginPath();
        ctx.arc(nx + 0.75, 6, 3 + blink * 1.5, 0, Math.PI * 2);
        ctx.fill();
      }

      for (const p of placed) {
        const age = t - p.born;
        if (age < 0) continue;
        const k = Math.min(1, age / 520);
        // Spring-ish drop: overshoot then settle.
        const s = 1 - Math.cos(k * Math.PI * 1.5) * Math.exp(-k * 5);
        const y = p.y - (1 - s) * 40;
        const overdue = p.day < 0;
        const hot = p.priority === 'urgent' || p.priority === 'high';
        const color = overdue ? pal.destructive : hot ? pal.signal : p.priority === 'medium' ? pal.fg : pal.muted;
        const size = p.id === hovered ? 11 : 8;
        if (overdue) {
          const ring = (t % 1600) / 1600;
          ctx.strokeStyle = hsl(pal.destructive, (1 - ring) * 0.6);
          ctx.lineWidth = 1;
          ctx.strokeRect(p.x - size / 2 - ring * 8, y - size / 2 - ring * 8, size + ring * 16, size + ring * 16);
        }
        ctx.globalAlpha = k;
        ctx.fillStyle = hsl(color);
        ctx.fillRect(p.x - size / 2, y - size / 2, size, size);
        ctx.globalAlpha = 1;
      }
    },
  };
}

export function DueStrip({ items, className }: { items: DueItem[]; className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const [hit, setHit] = useState<{ title: string; x: number; y: number; due: string } | null>(null);
  useScene(
    ref,
    () =>
      createDueScene(
        items,
        () => getComputedStyle(document.documentElement).getPropertyValue('--font-jetbrains-mono').trim() || 'monospace',
        setHit,
      ),
    { deps: [items.map((i) => i.id + i.due).join(',')] },
  );
  return (
    <div className={cn('relative', className)}>
      <canvas ref={ref} className="block h-full w-full" aria-hidden />
      {hit ? (
        <div
          role="tooltip"
          className="glass pointer-events-none absolute z-10 max-w-[240px] -translate-x-1/2 -translate-y-full animate-scale-in rounded-md px-2.5 py-1.5 text-xs"
          style={{ left: hit.x, top: hit.y - 12 }}
        >
          <p className="truncate font-medium">{hit.title}</p>
          <p className="font-mono text-[10px] text-muted-foreground">
            due {new Date(hit.due).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
          </p>
        </div>
      ) : null}
    </div>
  );
}

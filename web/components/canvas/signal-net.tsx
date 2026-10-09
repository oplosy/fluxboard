'use client';

import { useRef } from 'react';

import { cn } from '@/lib/utils';
import { hsl, rng, useScene, type Palette, type Scene } from './scene';

interface Node {
  label: string;
  sub: string;
  fx: number; // position as fraction of width/height
  fy: number;
  x: number;
  y: number;
  pulse: number;
}

interface Packet {
  from: number;
  to: number;
  t: number;
  speed: number;
  kind: 'write' | 'event';
}

/**
 * Realtime fan-out diagram: a client writes (cobalt) to the API, which
 * streams the change back to every other client over SSE (signal). Clicking
 * a client node sends a write from it.
 */
function createNetScene(getMono: () => string): Scene {
  const rand = rng(9);
  let W = 1;
  let H = 1;
  let palette: Palette | null = null;
  let since = 0;
  const packets: Packet[] = [];
  const nodes: Node[] = [
    { label: 'API', sub: 'SSE /events', fx: 0.5, fy: 0.5, x: 0, y: 0, pulse: 0 },
    { label: 'ada', sub: 'board', fx: 0.14, fy: 0.22, x: 0, y: 0, pulse: 0 },
    { label: 'lin', sub: 'list view', fx: 0.86, fy: 0.2, x: 0, y: 0, pulse: 0 },
    { label: 'sam', sub: 'task #142', fx: 0.16, fy: 0.8, x: 0, y: 0, pulse: 0 },
    { label: 'kai', sub: 'calendar', fx: 0.84, fy: 0.82, x: 0, y: 0, pulse: 0 },
  ];

  const place = () => {
    for (const n of nodes) {
      n.x = n.fx * W;
      n.y = n.fy * H;
    }
  };

  // Quadratic curve between two nodes, bowed for readability.
  const curve = (a: Node, b: Node, t: number) => {
    const mx = (a.x + b.x) / 2 + (b.y - a.y) * 0.18;
    const my = (a.y + b.y) / 2 - (b.x - a.x) * 0.18;
    const u = 1 - t;
    return { x: u * u * a.x + 2 * u * t * mx + t * t * b.x, y: u * u * a.y + 2 * u * t * my + t * t * b.y };
  };

  const write = (from: number) => {
    packets.push({ from, to: 0, t: 0, speed: 0.0011 + rand() * 0.0004, kind: 'write' });
  };

  return {
    resize(w, h) {
      W = w;
      H = h;
      place();
    },
    palette(p) {
      palette = p;
    },
    press(x, y) {
      let best = -1;
      let bestD = 60;
      nodes.forEach((n, i) => {
        const d = Math.hypot(n.x - x, n.y - y);
        if (i > 0 && d < bestD) {
          best = i;
          bestD = d;
        }
      });
      if (best > 0) write(best);
    },
    frame(ctx, _t, dt) {
      const pal = palette;
      if (!pal) return;
      ctx.clearRect(0, 0, W, H);
      since += dt;
      if (since > 1500) {
        since = 0;
        write(1 + Math.floor(rand() * (nodes.length - 1)));
      }

      const hub = nodes[0] as Node;
      ctx.setLineDash([3, 5]);
      ctx.lineWidth = 1;
      ctx.strokeStyle = hsl(pal.fg, 0.22);
      for (let i = 1; i < nodes.length; i++) {
        const n = nodes[i] as Node;
        ctx.beginPath();
        for (let s = 0; s <= 24; s++) {
          const p = curve(n, hub, s / 24);
          if (s === 0) ctx.moveTo(p.x, p.y);
          else ctx.lineTo(p.x, p.y);
        }
        ctx.stroke();
      }
      ctx.setLineDash([]);

      for (let i = packets.length - 1; i >= 0; i--) {
        const pk = packets[i] as Packet;
        pk.t += pk.speed * dt;
        const a = nodes[pk.from] as Node;
        const b = nodes[pk.to] as Node;
        const color = pk.kind === 'write' ? pal.cobalt : pal.signal;
        if (pk.t >= 1) {
          packets.splice(i, 1);
          b.pulse = 1;
          if (pk.kind === 'write') {
            for (let j = 1; j < nodes.length; j++) {
              if (j !== pk.from) packets.push({ from: 0, to: j, t: 0, speed: 0.0016 + rand() * 0.0005, kind: 'event' });
            }
          }
          continue;
        }
        // The curve is drawn client→hub, so outbound events run it backwards.
        const along = (s: number) => (pk.kind === 'write' ? curve(a, b, s) : curve(b, a, 1 - s));
        for (let s = 0; s < 7; s++) {
          const tt = Math.max(0, pk.t - s * 0.018);
          const p = along(tt);
          const size = s === 0 ? 6 : 4 - s * 0.4;
          ctx.fillStyle = hsl(color, s === 0 ? 1 : 0.5 - s * 0.06);
          ctx.fillRect(p.x - size / 2, p.y - size / 2, size, size);
        }
      }

      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      nodes.forEach((n, i) => {
        n.pulse = Math.max(0, n.pulse - dt / 700);
        const isHub = i === 0;
        const w = isHub ? 112 : 92;
        const h = isHub ? 52 : 44;
        if (n.pulse > 0) {
          ctx.strokeStyle = hsl(isHub ? pal.cobalt : pal.signal, n.pulse * 0.7);
          ctx.lineWidth = 1.5;
          const g = (1 - n.pulse) * 14;
          ctx.strokeRect(n.x - w / 2 - g, n.y - h / 2 - g, w + g * 2, h + g * 2);
        }
        ctx.fillStyle = isHub ? hsl(pal.fg) : hsl(pal.card);
        ctx.fillRect(n.x - w / 2, n.y - h / 2, w, h);
        ctx.strokeStyle = hsl(pal.fg, isHub ? 1 : 0.35);
        ctx.lineWidth = 1;
        ctx.strokeRect(n.x - w / 2 + 0.5, n.y - h / 2 + 0.5, w - 1, h - 1);
        ctx.fillStyle = isHub ? hsl(pal.bg) : hsl(pal.fg);
        ctx.font = `600 12px ${getMono()}`;
        ctx.fillText(isHub ? n.label : n.label.toUpperCase(), n.x, n.y - 7);
        ctx.fillStyle = isHub ? hsl(pal.bg, 0.6) : hsl(pal.fg, 0.5);
        ctx.font = `10px ${getMono()}`;
        ctx.fillText(n.sub, n.x, n.y + 9);
      });
    },
  };
}

export function SignalNet({ className }: { className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useScene(ref, () =>
    createNetScene(() => {
      const v = getComputedStyle(document.documentElement).getPropertyValue('--font-jetbrains-mono').trim();
      return v || 'monospace';
    }),
  );
  return <canvas ref={ref} className={cn('block h-full w-full cursor-crosshair', className)} aria-hidden />;
}

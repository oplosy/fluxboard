'use client';

import { useEffect, useRef, type CSSProperties } from 'react';

import { burstFrom } from '@/components/canvas/burst-layer';
import { useScrollProgress } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

const columns = ['Backlog', 'Doing', 'Review', 'Done'];
const WIP: Record<number, number> = { 2: 2 };

const cards = {
  c1: { key: 'FLX-31', title: 'Rate-limit API keys', who: 'AK', pri: 'High' },
  c2: { key: 'FLX-28', title: 'Invoice PDF export', who: 'LM', pri: 'Med' },
  c3: { key: 'FLX-35', title: 'Drag to reschedule', who: 'SO', pri: 'Low' },
  c4: { key: 'FLX-22', title: 'SSE resync on reconnect', who: 'KD', pri: 'High' },
  c5: { key: 'FLX-19', title: 'Audit log CSV export', who: 'AK', pri: 'Med' },
  c6: { key: 'FLX-40', title: 'Onboarding checklist', who: 'LM', pri: 'Low' },
} as const;
type CardId = keyof typeof cards;

// Board state per story step: column index → ordered card ids.
const states: CardId[][][] = [
  [['c6', 'c1', 'c3'], ['c4', 'c2'], ['c5'], []],
  [['c6', 'c3'], ['c4', 'c2', 'c1'], ['c5'], []],
  [['c6', 'c3'], ['c2', 'c1'], ['c5', 'c4'], []],
  [['c6', 'c3'], ['c2', 'c1'], ['c4'], ['c5']],
];

const steps = [
  {
    t: 'Capture',
    b: 'Type a title, press Enter. New cards get a rank between their neighbours, so the order you see survives every reload and every teammate.',
  },
  {
    t: 'Pull',
    b: 'Drag a card into Doing. The move applies instantly and quietly rolls back if someone else changed the board first.',
  },
  {
    t: 'Limit',
    b: 'Columns can carry a WIP limit. When Review is full the counter turns amber, before the queue becomes a problem.',
  },
  {
    t: 'Ship',
    b: 'Drop it into Done. Everyone with the board open sees it land at the same moment, without touching refresh.',
  },
];

function locate(step: number) {
  const pos = new Map<CardId, { col: number; row: number }>();
  (states[step] ?? []).forEach((ids, col) => ids.forEach((id, row) => pos.set(id, { col, row })));
  return pos;
}

/**
 * Scroll-driven product story: a sticky mini board whose cards travel between
 * columns as the reader scrolls through four steps.
 */
export function BoardStory() {
  const section = useRef<HTMLElement>(null);
  const progress = useScrollProgress(section);
  const step = Math.min(steps.length - 1, Math.floor(progress * steps.length * 1.02));
  const pos = locate(step);
  const prevStep = useRef(step);
  const doneCard = useRef<HTMLDivElement>(null);

  // Celebrate the ship step (only when arriving at it going forward).
  useEffect(() => {
    if (step === 3 && prevStep.current < 3) {
      const t = window.setTimeout(() => burstFrom(doneCard.current, 0.8), 450);
      prevStep.current = step;
      return () => window.clearTimeout(t);
    }
    prevStep.current = step;
  }, [step]);

  return (
    <section id="board" ref={section} className="relative h-[340vh] border-t border-border">
      <div className="sticky top-0 flex h-[100svh] items-center overflow-hidden">
        <div className="container grid items-center gap-10 lg:grid-cols-[0.8fr_1.2fr] lg:gap-16">
          <div>
            <p className="kicker">02 — The board</p>
            <h2 className="mt-4 text-[clamp(2.2rem,4.6vw,4rem)] font-semibold leading-[0.95] tracking-[-0.045em]">
              Four moves from idea to shipped.
            </h2>
            <ol className="mt-10 space-y-1">
              {steps.map((s, i) => {
                const active = i === step;
                return (
                  <li
                    key={s.t}
                    className={cn(
                      'relative border-l py-3 pl-5 transition-colors duration-500',
                      active ? 'border-signal' : 'border-border',
                    )}
                  >
                    <div className="flex items-baseline gap-3">
                      <span className="font-mono text-[11px] text-muted-foreground">0{i + 1}</span>
                      <span
                        className={cn(
                          'font-display text-xl font-semibold tracking-[-0.02em] transition-opacity duration-500',
                          active ? 'opacity-100' : 'opacity-35',
                        )}
                      >
                        {s.t}
                      </span>
                    </div>
                    <div
                      className={cn(
                        'grid transition-[grid-template-rows,opacity] duration-700 ease-out-expo',
                        active ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
                      )}
                    >
                      <p className="overflow-hidden pt-2 text-[15px] leading-relaxed text-muted-foreground">{s.b}</p>
                    </div>
                  </li>
                );
              })}
            </ol>
          </div>

          <div className="ticks relative rounded-lg border border-border bg-card/70 p-3 backdrop-blur-sm sm:p-4">
            <div className="mb-3 flex items-center justify-between px-1 font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
              <span>FLX / Platform board</span>
              <span className="flex items-center gap-1.5">
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-success" /> live
              </span>
            </div>
            <div className="relative grid grid-cols-4 gap-2" style={{ height: 380 }}>
              {columns.map((name, col) => {
                const count = states[step]?.[col]?.length ?? 0;
                const limit = WIP[col];
                const full = limit != null && count >= limit;
                return (
                  <div key={name} className="rounded-md bg-secondary/60 px-2 pt-2">
                    <div className="flex items-center justify-between px-1">
                      <span className="truncate text-[11px] font-semibold sm:text-xs">{name}</span>
                      <span
                        className={cn(
                          'rounded-sm px-1 font-mono text-[10px] transition-colors duration-500',
                          full ? 'bg-warning/25 text-foreground' : 'text-muted-foreground',
                        )}
                      >
                        {count}
                        {limit != null ? `/${limit}` : ''}
                      </span>
                    </div>
                  </div>
                );
              })}

              {(Object.keys(cards) as CardId[]).map((id) => {
                const p = pos.get(id);
                if (!p) return null;
                const c = cards[id];
                const isDone = p.col === 3;
                return (
                  <div
                    key={id}
                    ref={isDone ? doneCard : undefined}
                    className="absolute px-1 transition-[left,top] duration-700 ease-spring"
                    style={
                      {
                        left: `calc(${p.col} * 25%)`,
                        top: 34 + p.row * 82,
                        width: '25%',
                      } as CSSProperties
                    }
                  >
                    {/* Re-keyed per column so the hop animation replays on every move. */}
                    <div
                      key={`${id}-${p.col}`}
                      className={cn(
                        'rounded-md border bg-card p-2 shadow-[0_6px_18px_-10px_hsl(var(--ink)/0.4)] sm:p-2.5',
                        'animate-[card-hop_0.7s_var(--ease-out-expo)]',
                        isDone ? 'border-signal/60' : 'border-border',
                      )}
                    >
                      <p className="line-clamp-2 text-[11px] font-medium leading-snug sm:text-[13px]">{c.title}</p>
                      <div className="mt-2 flex items-center justify-between">
                        <span className="font-mono text-[9px] text-muted-foreground sm:text-[10px]">{c.key}</span>
                        <span
                          className={cn(
                            'hidden h-5 w-5 items-center justify-center rounded-full font-mono text-[8px] font-medium text-white sm:flex',
                            isDone ? 'bg-signal text-signal-foreground' : 'bg-foreground/70',
                          )}
                        >
                          {c.who}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
            <div className="mt-3 h-[2px] overflow-hidden rounded-full bg-secondary">
              <div className="h-full origin-left bg-signal" style={{ transform: `scaleX(${progress})` }} />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

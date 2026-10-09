'use client';

import Link from 'next/link';
import { useState } from 'react';
import { ArrowDown, ArrowUpRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { FluxField, FLUX_LANES } from '@/components/canvas/flux-field';
import { Magnetic, ScrambleText } from '@/components/motion/effects';
import { Reveal, SplitText } from '@/components/motion/reveal';

const laneNames = ['Backlog', 'Doing', 'Review', 'Done'];

const specs = [
  { n: '01', k: 'Ordering', v: 'LexoRank, conflict-safe' },
  { n: '02', k: 'Realtime', v: 'SSE fan-out per org' },
  { n: '03', k: 'Isolation', v: 'Postgres row-level security' },
  { n: '04', k: 'Billing', v: 'Stripe, metered usage' },
];

export function Hero() {
  const [shipped, setShipped] = useState(0);

  return (
    <section className="grain relative isolate flex min-h-[100svh] flex-col overflow-hidden pt-[72px]">
      {/* Flow field: masked so it fades out behind the headline. */}
      <div className="blueprint absolute inset-0 -z-20 opacity-60 [mask-image:radial-gradient(ellipse_at_70%_45%,#000_20%,transparent_75%)]" />
      <div className="absolute inset-0 -z-10 [mask-image:linear-gradient(180deg,transparent_35%,#000_95%)] md:[mask-image:linear-gradient(90deg,transparent_24%,#000_60%)]">
        <FluxField onShip={() => setShipped((n) => n + 1)} />
      </div>

      {/* Lane gates + labels, aligned with the canvas lanes. */}
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 hidden md:block">
        {FLUX_LANES.slice(0, -1).map((f, i) => (
          <div key={f} className="absolute bottom-0 top-[72px] border-l border-dashed border-foreground/15" style={{ left: `${f * 100}%` }}>
            <span className="absolute left-3 top-6 font-mono text-[10px] uppercase tracking-[0.2em] text-muted-foreground">
              <span className={i === 3 ? 'text-signal-ink' : undefined}>
                <ScrambleText text={`${String(i + 1).padStart(2, '0')} ${laneNames[i]}`} />
              </span>
            </span>
          </div>
        ))}
      </div>

      <div className="container flex flex-1 flex-col justify-center py-16">
        <Reveal variant="fade" className="kicker mb-8 flex items-center gap-3">
          <span className="h-px w-8 bg-foreground/40" />
          Project workspace for small teams
        </Reveal>

        <h1 className="max-w-[13ch] text-[clamp(3rem,8.4vw,7.6rem)] font-semibold leading-[0.9] tracking-[-0.055em]">
          <SplitText text="The board moves before you refresh." wordClassName={(w) => (w === 'moves' ? 'text-signal' : undefined)} />
        </h1>

        <Reveal index={3} className="mt-8 max-w-[46ch] text-lg leading-relaxed text-muted-foreground">
          Kanban with real ordering, live updates over server-sent events, roles that isolate every tenant, and
          billing that meters itself. One workspace, no plugins.
        </Reveal>

        <Reveal index={4} className="mt-10 flex flex-wrap items-center gap-3">
          <Magnetic>
            <Link href="/register" tabIndex={-1}>
              <Button variant="signal" size="xl" className="gap-2.5">
                <span className="roll">
                  <span>Start a workspace</span>
                  <span aria-hidden>It&apos;s free</span>
                </span>
                <ArrowUpRight className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-45" />
              </Button>
            </Link>
          </Magnetic>
          <a
            href="#board"
            className="group flex h-14 items-center gap-2 px-4 text-[15px] text-muted-foreground transition-colors hover:text-foreground"
            data-transition="none"
          >
            <span className="link-underline">See it work</span>
            <ArrowDown className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:translate-y-1" />
          </a>
        </Reveal>
      </div>

      <div className="container relative pb-6">
        <div className="flex items-end justify-between gap-6 border-t border-border pt-5">
          <dl className="grid flex-1 grid-cols-2 gap-x-6 gap-y-4 md:grid-cols-4">
            {specs.map((s, i) => (
              <Reveal key={s.n} index={i + 5} variant="up">
                <dt className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">
                  {s.n} / {s.k}
                </dt>
                <dd className="mt-1.5 text-sm font-medium">{s.v}</dd>
              </Reveal>
            ))}
          </dl>
          <div className="hidden shrink-0 text-right lg:block" aria-live="off">
            <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Shipped in this field</p>
            <p className="tabular font-display text-4xl font-semibold tracking-[-0.04em]">
              {String(shipped).padStart(4, '0')}
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

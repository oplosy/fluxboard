'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowUpRight, Fingerprint, KeyRound, Layers3, ScrollText } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { SignalNet } from '@/components/canvas/signal-net';
import { CountUp, Magnetic, Marquee, ScrambleText } from '@/components/motion/effects';
import { Reveal, SplitText } from '@/components/motion/reveal';
import { useInView } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

// ── Ticker ──────────────────────────────────────────────────────────────

const facts = [
  'task.moved → Review',
  'WIP 2/2 · Review',
  'invoice.finalized · $12.00',
  'member.invited · role=MEMBER',
  'rank a0V → a0Vh',
  'sse: resync after 4.2s',
  'audit: api_key.revoked',
  '2FA · TOTP verified',
  'usage: api_calls +1',
];

export function Ticker() {
  return (
    <div className="border-y border-border bg-foreground py-3 text-background">
      <Marquee duration={50}>
        {facts.map((f) => (
          <span key={f} className="flex items-center gap-6 pr-6 font-mono text-xs uppercase tracking-[0.14em]">
            {f}
            <span className="h-1 w-1 rounded-full bg-signal" />
          </span>
        ))}
      </Marquee>
    </div>
  );
}

// ── Section header ──────────────────────────────────────────────────────

function SectionHead({ n, kicker, title, body }: { n: string; kicker: string; title: string; body: string }) {
  return (
    <div className="grid gap-6 lg:grid-cols-[0.9fr_1.1fr] lg:items-end">
      <div>
        <Reveal variant="fade" className="kicker">
          {n} — {kicker}
        </Reveal>
        <h2 className="mt-4 text-[clamp(2.2rem,4.6vw,4rem)] font-semibold leading-[0.95] tracking-[-0.045em]">
          <SplitText text={title} />
        </h2>
      </div>
      <Reveal index={2} className="max-w-[48ch] text-[17px] leading-relaxed text-muted-foreground lg:justify-self-end">
        {body}
      </Reveal>
    </div>
  );
}

// ── Realtime ────────────────────────────────────────────────────────────

const log = [
  ['event', 'task.created', '{"key":"FLX-41","col":"Backlog"}'],
  ['event', 'task.moved', '{"key":"FLX-31","to":"Doing"}'],
  ['event', 'comment.added', '{"key":"FLX-22","by":"kai"}'],
  ['event', 'task.moved', '{"key":"FLX-19","to":"Done"}'],
  [':', 'heartbeat', ''],
  ['event', 'label.attached', '{"key":"FLX-28","label":"billing"}'],
];

function EventLog() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: false });
  const [lines, setLines] = useState(1);

  useEffect(() => {
    if (!inView) return;
    const id = window.setInterval(() => setLines((n) => (n >= log.length ? 1 : n + 1)), 1300);
    return () => window.clearInterval(id);
  }, [inView]);

  return (
    <div ref={ref} className="rounded-md border border-border bg-card font-mono text-[11.5px] leading-6">
      <div className="flex items-center gap-1.5 border-b border-border px-3 py-2 text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="ml-2">GET /orgs/:id/events</span>
      </div>
      <div className="h-[168px] overflow-hidden px-3 py-2">
        {log.slice(0, lines).map(([a, b, c], i) => (
          <p key={`${i}-${lines > i}`} className="animate-slide-up truncate">
            <span className="text-muted-foreground">{a}:</span> <span className="text-signal-ink">{b}</span>
            {c ? <span className="text-muted-foreground"> {c}</span> : null}
          </p>
        ))}
        <span className="inline-block h-3.5 w-1.5 translate-y-0.5 animate-blink bg-foreground" />
      </div>
    </div>
  );
}

export function RealtimeSection() {
  return (
    <section className="border-t border-border py-24 md:py-36">
      <div className="container">
        <SectionHead
          n="03"
          kicker="Realtime"
          title="One write, every screen."
          body="Each change goes to the API once and streams back to every open board, list and calendar in the organisation over server-sent events. Click a client in the diagram to send a write."
        />
        <div className="mt-14 grid grid-cols-1 gap-6 lg:grid-cols-[1.4fr_0.6fr]">
          <Reveal variant="scale" className="ticks relative h-[360px] rounded-lg border border-border bg-card/50 md:h-[440px]">
            <SignalNet />
            <div className="pointer-events-none absolute bottom-3 left-3 flex gap-4 font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 bg-cobalt" /> write
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 bg-signal" /> event
              </span>
            </div>
          </Reveal>
          <div className="flex flex-col gap-6">
            <Reveal index={1}>
              <EventLog />
            </Reveal>
            <Reveal index={2} className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border bg-border">
              {[
                ['Backoff', '1 → 10 s'],
                ['Auth', 'Bearer, in memory'],
                ['Resume', 'Last-Event-ID'],
                ['Cache', 'Surgical invalidation'],
              ].map(([k, v]) => (
                <div key={k} className="bg-card p-3">
                  <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">{k}</p>
                  <p className="mt-1 text-sm font-medium">{v}</p>
                </div>
              ))}
            </Reveal>
          </div>
        </div>
      </div>
    </section>
  );
}

// ── Billing ─────────────────────────────────────────────────────────────

const meters = [
  { k: 'Seats', used: 7, limit: 15, unit: '' },
  { k: 'Storage', used: 3.2, limit: 10, unit: ' GB' },
  { k: 'API calls', used: 41_380, limit: 50_000, unit: '' },
];

function Meter({ k, used, limit, unit, index }: { k: string; used: number; limit: number; unit: string; index: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref);
  const pct = used / limit;
  return (
    <div ref={ref}>
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-medium">{k}</span>
        <span className="font-mono text-xs text-muted-foreground">
          <CountUp value={used} decimals={unit === ' GB' ? 1 : 0} suffix={unit} /> / {limit.toLocaleString('en-US')}
          {unit}
        </span>
      </div>
      <div className="mt-2 grid grid-cols-[repeat(40,1fr)] gap-[2px]">
        {Array.from({ length: 40 }).map((_, i) => {
          const on = i / 40 < pct;
          return (
            <span
              key={i}
              className={cn(
                'h-5 rounded-[1px] transition-colors duration-300',
                inView && on ? (pct > 0.8 && i / 40 > 0.75 ? 'bg-signal' : 'bg-foreground') : 'bg-secondary',
              )}
              style={{ transitionDelay: `${index * 200 + i * 22}ms` }}
            />
          );
        })}
      </div>
    </div>
  );
}

export function BillingSection() {
  return (
    <section className="border-t border-border py-24 md:py-36">
      <div className="container">
        <SectionHead
          n="04"
          kicker="Billing"
          title="It meters itself."
          body="Seats, storage and API calls are counted as they happen and reported to Stripe. Hit a limit and you get an upgrade prompt, not a silent failure. Reads are never blocked."
        />
        <div className="mt-14 grid grid-cols-1 gap-6 lg:grid-cols-2">
          <Reveal variant="up" className="space-y-7 rounded-lg border border-border bg-card p-6 md:p-8">
            <div className="flex items-center justify-between">
              <p className="kicker">Usage · this period</p>
              <span className="rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider">Pro</span>
            </div>
            {meters.map((m, i) => (
              <Meter key={m.k} {...m} index={i} />
            ))}
          </Reveal>
          <Reveal variant="up" index={1} className="flex flex-col rounded-lg border border-border bg-card p-6 md:p-8">
            <div className="flex items-center justify-between">
              <p className="kicker">Example invoice</p>
              <span className="font-mono text-[10px] text-muted-foreground">INV-0042</span>
            </div>
            <dl className="mt-6 flex-1 divide-y divide-dashed divide-border font-mono text-sm">
              {[
                ['Pro plan · monthly', 12],
                ['Extra seats · 0', 0],
                ['Storage overage · 0 GB', 0],
                ['API calls over allowance', 0],
              ].map(([label, amount], i) => (
                <Reveal key={String(label)} as="div" variant="clip" index={i + 2} className="flex justify-between py-3">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="tabular">${Number(amount).toFixed(2)}</dd>
                </Reveal>
              ))}
            </dl>
            <div className="mt-4 flex items-end justify-between border-t border-foreground pt-4">
              <span className="font-mono text-xs uppercase tracking-[0.16em] text-muted-foreground">Total due</span>
              <span className="font-display text-5xl font-semibold tracking-[-0.04em]">
                <CountUp value={12} decimals={2} prefix="$" duration={1800} />
              </span>
            </div>
          </Reveal>
        </div>
      </div>
    </section>
  );
}

// ── Security ────────────────────────────────────────────────────────────

const guards = [
  {
    icon: Layers3,
    t: 'Row-level security',
    b: 'Every tenant table is filtered by Postgres RLS and again in the application layer.',
  },
  { icon: Fingerprint, t: 'TOTP two-factor', b: 'Authenticator-app 2FA with recovery codes; required for platform admins.' },
  { icon: KeyRound, t: 'Token rotation', b: 'Short-lived access tokens kept in memory, refresh tokens rotated in httpOnly cookies.' },
  { icon: ScrollText, t: 'Audit trail', b: 'Org-scoped audit log with severity, actor filters and CSV export.' },
];

export function SecuritySection() {
  return (
    <section className="border-t border-border py-24 md:py-36">
      <div className="container">
        <SectionHead
          n="05"
          kicker="Boundaries"
          title="Isolation is not a setting."
          body="Tenancy, roles and audit history are part of the foundation, not an enterprise add-on. Owner, Admin, Member and Guest each see exactly what they should."
        />
        <div className="mt-14 grid gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-2 lg:grid-cols-4">
          {guards.map((g, i) => (
            <Reveal key={g.t} index={i} className="group relative bg-background p-6 transition-colors duration-500 hover:bg-card">
              <g.icon className="h-5 w-5 transition-[transform,color] duration-500 ease-spring group-hover:-rotate-6 group-hover:scale-110 group-hover:text-signal" strokeWidth={1.6} />
              <h3 className="mt-10 font-mono text-[13px] font-semibold uppercase tracking-[0.08em]">
                <ScrambleText text={g.t} replayOnHover />
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{g.b}</p>
              <span className="absolute inset-x-0 bottom-0 h-[2px] origin-left scale-x-0 bg-signal transition-transform duration-500 ease-out-expo group-hover:scale-x-100" />
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

// ── Closing CTA ─────────────────────────────────────────────────────────

export function ClosingCta() {
  return (
    <section className="relative overflow-hidden bg-ink py-28 text-paper md:py-40">
      <div className="pointer-events-none absolute inset-0 opacity-[0.07] [background-image:linear-gradient(hsl(var(--paper))_1px,transparent_1px),linear-gradient(90deg,hsl(var(--paper))_1px,transparent_1px)] [background-size:48px_48px]" />
      <div className="container relative grid gap-12 md:grid-cols-[1fr_auto] md:items-end">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-paper/50">06 — Start</p>
          <h2 className="mt-5 max-w-[14ch] text-[clamp(2.6rem,7vw,6.5rem)] font-semibold leading-[0.9] tracking-[-0.055em]">
            <SplitText text="Your next standup could be shorter." wordClassName={(w) => (w === 'shorter.' ? 'text-signal' : undefined)} />
          </h2>
        </div>
        <Reveal index={3} className="flex flex-col items-start gap-4">
          <Magnetic strength={0.35}>
            <Link href="/register" tabIndex={-1}>
              <Button variant="signal" size="xl" className="gap-2.5">
                <span className="roll">
                  <span>Create a workspace</span>
                  <span aria-hidden>Takes a minute</span>
                </span>
                <ArrowUpRight className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-45" />
              </Button>
            </Link>
          </Magnetic>
          <p className="font-mono text-[11px] text-paper/50">Free plan · no card required</p>
        </Reveal>
      </div>
    </section>
  );
}

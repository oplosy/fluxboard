'use client';

import { useRef } from 'react';

import { Reveal } from '@/components/motion/reveal';
import { useScrollProgress } from '@/lib/motion/hooks';

export interface ChangelogEntry {
  date: string;
  version: string;
  changes: string[];
}

/** Vertical timeline whose spine draws itself as you scroll through it. */
export function ChangelogTimeline({ entries }: { entries: ChangelogEntry[] }) {
  const ref = useRef<HTMLDivElement>(null);
  const progress = useScrollProgress(ref);

  return (
    <section className="container pb-28">
      <div ref={ref} className="relative mx-auto max-w-4xl">
        <span aria-hidden className="absolute bottom-0 left-[7px] top-0 w-px bg-border md:left-[183px]" />
        <span
          aria-hidden
          className="absolute left-[7px] top-0 w-px origin-top bg-signal md:left-[183px]"
          style={{ height: '100%', transform: `scaleY(${Math.min(1, progress * 1.25 + 0.05)})` }}
        />
        <div className="space-y-16">
          {entries.map((e, i) => (
            <article key={e.date} className="relative grid gap-4 pl-10 md:grid-cols-[160px_1fr] md:gap-12 md:pl-0">
              <Reveal variant="fade" className="font-mono text-xs uppercase tracking-[0.14em] text-muted-foreground md:pt-1.5 md:text-right">
                <time dateTime={e.date}>{e.date}</time>
              </Reveal>
              <span aria-hidden className="absolute left-0 top-1 flex h-[15px] w-[15px] items-center justify-center md:left-[176px]">
                {i === 0 ? <span className="absolute h-full w-full animate-pulse-ring rounded-full bg-signal" /> : null}
                <span className="h-[9px] w-[9px] rounded-full border-2 border-background bg-foreground ring-1 ring-foreground" />
              </span>
              <div className="md:pl-6">
                <Reveal as="h2" className="text-3xl font-semibold tracking-[-0.035em]">
                  {e.version}
                </Reveal>
                <ul className="mt-5 space-y-3">
                  {e.changes.map((c, j) => (
                    <Reveal as="li" key={c} index={j + 1} className="flex gap-3 text-[15px] leading-relaxed text-muted-foreground">
                      <span className="mt-[0.7em] h-px w-3 shrink-0 bg-foreground/40" />
                      {c}
                    </Reveal>
                  ))}
                </ul>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

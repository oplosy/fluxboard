'use client';

import { useId, useState } from 'react';

import { Reveal } from '@/components/motion/reveal';
import { cn } from '@/lib/utils';

/** Accordion with animated height (grid-rows) and a plus that turns into a minus. */
export function Faq({ items }: { items: { q: string; a: string }[] }) {
  const [open, setOpen] = useState<number | null>(0);
  const base = useId();
  return (
    <dl className="border-t border-foreground">
      {items.map((f, i) => {
        const isOpen = open === i;
        const id = `${base}-${i}`;
        return (
          <Reveal key={f.q} index={i} className="border-b border-border">
            <dt>
              <button
                type="button"
                aria-expanded={isOpen}
                aria-controls={id}
                onClick={() => setOpen(isOpen ? null : i)}
                className="group flex w-full items-center justify-between gap-6 py-5 text-left"
              >
                <span className="text-lg font-medium tracking-[-0.01em] transition-colors group-hover:text-signal-ink">{f.q}</span>
                <span className="relative h-3.5 w-3.5 shrink-0" aria-hidden>
                  <span className="absolute left-0 top-1/2 h-[1.5px] w-full -translate-y-1/2 bg-current" />
                  <span
                    className={cn(
                      'absolute left-1/2 top-0 h-full w-[1.5px] -translate-x-1/2 bg-current transition-transform duration-500 ease-spring',
                      isOpen && 'scale-y-0 rotate-90',
                    )}
                  />
                </span>
              </button>
            </dt>
            <dd
              id={id}
              className={cn('grid transition-[grid-template-rows] duration-500 ease-out-expo', isOpen ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]')}
            >
              <p
                className={cn(
                  'overflow-hidden pr-10 leading-relaxed text-muted-foreground transition-opacity duration-500',
                  isOpen ? 'pb-6 opacity-100' : 'opacity-0',
                )}
              >
                {f.a}
              </p>
            </dd>
          </Reveal>
        );
      })}
    </dl>
  );
}

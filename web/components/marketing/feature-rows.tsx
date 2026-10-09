'use client';

import { useState, type ReactNode } from 'react';
import { Plus } from 'lucide-react';

import { Reveal } from '@/components/motion/reveal';
import { cn } from '@/lib/utils';

interface Group {
  icon: ReactNode;
  title: string;
  body: string;
  tags: string[];
}

/**
 * Numbered feature index. Rows open like an accordion (height animated via
 * grid rows); hovering a row sweeps a fill across it.
 */
export function FeatureRows({ groups }: { groups: Group[] }) {
  const [open, setOpen] = useState(0);
  return (
    <section className="container py-20 md:py-28">
      <ul className="border-t border-foreground">
        {groups.map((g, i) => {
          const isOpen = open === i;
          return (
            <Reveal as="li" key={g.title} index={i % 4} className="group relative isolate border-b border-border">
              <span
                aria-hidden
                className="absolute inset-0 -z-10 origin-left scale-x-0 bg-secondary/70 transition-transform duration-700 ease-out-expo group-hover:scale-x-100"
              />
              <button
                type="button"
                onClick={() => setOpen(isOpen ? -1 : i)}
                aria-expanded={isOpen}
                className="grid w-full grid-cols-[3rem_1fr_auto] items-center gap-4 py-6 text-left md:grid-cols-[5rem_auto_1fr_auto] md:py-8"
              >
                <span className="font-mono text-xs text-muted-foreground">{String(i + 1).padStart(2, '0')}</span>
                <span className="hidden text-muted-foreground transition-colors duration-500 group-hover:text-signal md:block">{g.icon}</span>
                <span className="font-display text-2xl font-semibold tracking-[-0.03em] transition-transform duration-500 ease-out-expo group-hover:translate-x-2 md:text-4xl">
                  {g.title}
                </span>
                <span
                  className={cn(
                    'flex h-9 w-9 items-center justify-center rounded-full border transition-[transform,background-color,border-color,color] duration-500 ease-spring',
                    isOpen ? 'rotate-45 border-foreground bg-foreground text-background' : 'border-border',
                  )}
                >
                  <Plus className="h-4 w-4" />
                </span>
              </button>
              <div
                className={cn(
                  'grid transition-[grid-template-rows] duration-700 ease-out-expo',
                  isOpen ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]',
                )}
              >
                <div className="overflow-hidden">
                  <div className="grid gap-6 pb-8 md:grid-cols-[5rem_1fr_1fr] md:pb-10">
                    <span />
                    <p
                      className={cn(
                        'max-w-[52ch] text-[17px] leading-relaxed text-muted-foreground transition-[opacity,transform] duration-700 ease-out-expo',
                        isOpen ? 'translate-y-0 opacity-100 delay-150' : 'translate-y-3 opacity-0',
                      )}
                    >
                      {g.body}
                    </p>
                    <ul className="flex flex-wrap content-start gap-2">
                      {g.tags.map((t, j) => (
                        <li
                          key={t}
                          className={cn(
                            'rounded-sm border border-border px-2 py-1 font-mono text-[11px] uppercase tracking-[0.1em] transition-[opacity,transform] duration-500 ease-spring',
                            isOpen ? 'scale-100 opacity-100' : 'scale-90 opacity-0',
                          )}
                          style={{ transitionDelay: isOpen ? `${220 + j * 60}ms` : '0ms' }}
                        >
                          {t}
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>
              </div>
            </Reveal>
          );
        })}
      </ul>
    </section>
  );
}

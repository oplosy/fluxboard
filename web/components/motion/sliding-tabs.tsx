'use client';

import Link from 'next/link';
import { useLayoutEffect, useRef, useState } from 'react';

import { cn } from '@/lib/utils';

export interface SlidingTab {
  href: string;
  label: string;
  active: boolean;
}

/**
 * Link tabs with an underline that glides to the active tab (and previews
 * the hovered one). Used for project, settings and billing sub-navigation.
 */
export function SlidingTabs({ tabs, className, vertical = false }: { tabs: SlidingTab[]; className?: string; vertical?: boolean }) {
  const wrap = useRef<HTMLElement>(null);
  const refs = useRef<(HTMLAnchorElement | null)[]>([]);
  const [hover, setHover] = useState<number | null>(null);
  const [box, setBox] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
  const activeIndex = tabs.findIndex((t) => t.active);
  const target = hover ?? activeIndex;

  useLayoutEffect(() => {
    const el = target >= 0 ? refs.current[target] : null;
    if (!el) {
      setBox(null);
      return;
    }
    setBox({ x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight });
  }, [target, tabs.length]);

  return (
    <nav
      ref={wrap}
      className={cn('relative flex', vertical ? 'flex-row gap-1 overflow-x-auto md:flex-col' : 'gap-1 overflow-x-auto', className)}
      onMouseLeave={() => setHover(null)}
    >
      {box ? (
        <span
          aria-hidden
          className={cn(
            'pointer-events-none absolute rounded-md transition-[transform,width,height,background-color] duration-400 ease-out-expo',
            hover !== null && hover !== activeIndex ? 'bg-secondary/70' : 'bg-secondary',
          )}
          style={{ transform: `translate(${box.x}px, ${box.y}px)`, width: box.w, height: box.h, left: 0, top: 0 }}
        />
      ) : null}
      {tabs.map((t, i) => (
        <Link
          key={t.href}
          href={t.href}
          ref={(el) => {
            refs.current[i] = el;
          }}
          onMouseEnter={() => setHover(i)}
          aria-current={t.active ? 'page' : undefined}
          className={cn(
            'relative z-[1] whitespace-nowrap rounded-md px-3 py-2 text-sm transition-colors duration-200',
            t.active ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {t.label}
          {t.active ? (
            <span
              className={cn(
                'absolute bg-signal',
                vertical ? 'bottom-1 left-3 right-3 h-[2px] md:bottom-auto md:left-0 md:right-auto md:top-1/2 md:h-4 md:w-[2px] md:-translate-y-1/2' : 'bottom-0 left-3 right-3 h-[2px]',
              )}
            />
          ) : null}
        </Link>
      ))}
    </nav>
  );
}

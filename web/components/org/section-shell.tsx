'use client';

import type { ReactNode } from 'react';

import { SlidingTabs, type SlidingTab } from '@/components/motion/sliding-tabs';

/**
 * Two-column shell for settings-like areas: heading, a vertical sliding-tab
 * nav and the page body. `locked` replaces the body with a notice.
 */
export function SectionShell({
  kicker,
  title,
  tabs,
  locked,
  action,
  children,
}: {
  kicker: string;
  title: string;
  tabs: SlidingTab[];
  locked?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="mx-auto max-w-6xl px-5 py-8 lg:px-10 lg:py-12">
      <div className="mb-10 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="kicker animate-fade-in">{kicker}</p>
          <h1 className="mt-3 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-4xl font-semibold tracking-[-0.05em] sm:text-5xl">
            {title}
          </h1>
        </div>
        {action}
      </div>
      {locked ? (
        <p className="max-w-2xl animate-fade-in rounded-lg border border-border bg-card p-5 text-sm text-muted-foreground">{locked}</p>
      ) : (
        <div className="grid gap-8 md:grid-cols-[190px_1fr] md:gap-12">
          <div className="md:sticky md:top-20 md:self-start">
            <SlidingTabs vertical tabs={tabs} />
          </div>
          <div className="min-w-0 animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] [animation-delay:120ms]">{children}</div>
        </div>
      )}
    </div>
  );
}

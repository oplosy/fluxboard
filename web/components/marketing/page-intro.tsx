import type { ReactNode } from 'react';

import { Reveal, SplitText } from '@/components/motion/reveal';
import { cn } from '@/lib/utils';

/** Shared opening block for marketing sub-pages (clears the fixed header). */
export function PageIntro({
  kicker,
  title,
  lede,
  aside,
  className,
}: {
  kicker: string;
  title: string;
  lede?: ReactNode;
  aside?: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn('container pb-14 pt-36 md:pb-20 md:pt-44', className)}>
      <Reveal variant="fade" className="kicker flex items-center gap-3">
        <span className="h-px w-8 bg-foreground/40" />
        {kicker}
      </Reveal>
      <div className="mt-6 grid gap-8 lg:grid-cols-[1.2fr_0.8fr] lg:items-end">
        <h1 className="text-[clamp(2.8rem,7.5vw,6.5rem)] font-semibold leading-[0.9] tracking-[-0.055em]">
          <SplitText text={title} />
        </h1>
        {lede || aside ? (
          <Reveal index={3} className="max-w-[46ch] text-[17px] leading-relaxed text-muted-foreground lg:justify-self-end">
            {lede}
            {aside}
          </Reveal>
        ) : null}
      </div>
    </section>
  );
}

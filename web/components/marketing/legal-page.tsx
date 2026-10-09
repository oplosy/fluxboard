import type { ReactNode } from 'react';

import { PageIntro } from '@/components/marketing/page-intro';
import { Reveal } from '@/components/motion/reveal';

// Shared shell for the legal pages: a readable prose column under the intro.
export function LegalPage({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <>
      <PageIntro kicker={`Legal · updated ${updated}`} title={title} />
      <section className="container pb-28">
        <Reveal
          variant="fade"
          className="mx-auto max-w-[68ch] space-y-4 border-t border-foreground pt-10 text-[15px] leading-[1.75] text-muted-foreground [&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-4 [&_h2]:mt-12 [&_h2]:font-display [&_h2]:text-2xl [&_h2]:font-semibold [&_h2]:tracking-[-0.02em] [&_h2]:text-foreground [&_li]:ml-5 [&_li]:list-disc [&_strong]:text-foreground"
        >
          {children}
        </Reveal>
      </section>
    </>
  );
}

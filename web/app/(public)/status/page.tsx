import type { Metadata } from 'next';

import { PageIntro } from '@/components/marketing/page-intro';
import { Reveal } from '@/components/motion/reveal';

export const metadata: Metadata = { title: 'Status' };

const systems = ['API', 'Web app', 'Realtime (SSE)', 'Background jobs', 'Billing'];

export default function StatusPage() {
  return (
    <>
      <PageIntro kicker="Status" title="System status" />
      <section className="container max-w-4xl pb-28">
        <Reveal variant="scale" className="flex items-center gap-4 rounded-lg border border-success/30 bg-success/[0.06] p-5">
          <span className="relative flex h-3 w-3">
            <span className="absolute inset-0 animate-pulse-ring rounded-full bg-success" />
            <span className="relative h-3 w-3 rounded-full bg-success" />
          </span>
          <p className="font-medium">All systems operational</p>
        </Reveal>
        <ul className="mt-8 border-t border-foreground">
          {systems.map((s, i) => (
            <Reveal as="li" key={s} index={i} className="group flex items-center justify-between border-b border-border py-5">
              <span className="flex items-center gap-4">
                <span className="font-mono text-xs text-muted-foreground">{String(i + 1).padStart(2, '0')}</span>
                <span className="text-lg font-medium tracking-[-0.01em] transition-transform duration-500 ease-out-expo group-hover:translate-x-1">
                  {s}
                </span>
              </span>
              <span className="flex items-center gap-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
                <span className="h-2 w-2 rounded-full bg-success" />
                Operational
              </span>
            </Reveal>
          ))}
        </ul>
        <p className="mt-8 text-sm text-muted-foreground">
          This is a placeholder status page. In production it would link to a dedicated status provider with
          historical uptime.
        </p>
      </section>
    </>
  );
}

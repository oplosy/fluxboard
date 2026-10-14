'use client';

import Link from 'next/link';
import { ArrowUpRight, Check } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { CountUp, Tilt } from '@/components/motion/effects';
import { Reveal } from '@/components/motion/reveal';
import { cn } from '@/lib/utils';
import { marketingCopy } from '@/lib/marketing/plans';
import { limitLabel, usePlanCatalog } from '@/lib/billing/plans';
import { formatBytes } from '@/lib/billing/format';

// Prices and limits come from the server catalog (GET /plans, report B02); this
// component only adds copy, so the public page shows exactly what is enforced.
export function PricingTable() {
  const catalog = usePlanCatalog();

  if (catalog.isError) {
    return (
      <p className="rounded-lg border border-border p-6 text-sm text-muted-foreground">
        Pricing is temporarily unavailable. Please try again in a moment.
      </p>
    );
  }
  if (!catalog.data) {
    return (
      <div className="grid gap-4 md:grid-cols-3" aria-busy="true">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-[30rem] animate-pulse rounded-lg border border-border bg-card" />
        ))}
      </div>
    );
  }

  const plans = catalog.data.plans.map((p) => {
    const copy = marketingCopy[p.code];
    return {
      key: p.code,
      name: p.name,
      seatPrice: p.seat_price,
      tagline: copy?.tagline ?? '',
      highlighted: copy?.highlighted ?? false,
      features: [
        `${limitLabel(p.max_projects)} ${p.max_projects === 1 ? 'project' : 'projects'}`,
        `${limitLabel(p.max_members)} members`,
        `${limitLabel(p.max_agents)} ${p.max_agents === 1 ? 'agent' : 'agents'} over MCP`,
        `${formatBytes(p.max_storage_bytes)} storage`,
        `${p.api_rate_per_min.toLocaleString()} API requests / min`,
        `${p.audit_retention_days}-day audit log`,
        ...(p.metered ? ['Storage and API usage metering'] : []),
        ...(copy?.extras ?? []),
      ],
    };
  });

  return (
    <div className="grid gap-4 md:grid-cols-3">
      {plans.map((plan, i) => (
        <Reveal key={plan.key} index={i} variant="up" className="h-full">
          <Tilt
            max={5}
            className={cn(
              'flex h-full flex-col rounded-lg border p-6 transition-[border-color] duration-500 md:p-7',
              plan.highlighted ? 'border-foreground bg-foreground text-background' : 'border-border bg-card hover:border-foreground/40',
            )}
          >
            <div className="flex items-center justify-between">
              <span className="font-mono text-xs opacity-60">0{i + 1}</span>
              {plan.highlighted ? (
                <span className="rounded-sm bg-signal px-1.5 py-0.5 text-xs font-medium text-signal-foreground">
                  Most teams
                </span>
              ) : null}
            </div>
            <h3 className="mt-8 font-display text-2xl font-semibold tracking-[-0.03em]">{plan.name}</h3>
            <p className={cn('mt-1 text-sm', plan.highlighted ? 'text-background/60' : 'text-muted-foreground')}>{plan.tagline}</p>
            <p className="mt-8 flex items-baseline gap-1">
              <span className="font-display text-6xl font-semibold tracking-[-0.05em]">
                <CountUp value={plan.seatPrice / 100} prefix="$" duration={1200 + i * 200} />
              </span>
              <span className={cn('text-sm', plan.highlighted ? 'text-background/60' : 'text-muted-foreground')}>
                {plan.seatPrice > 0 ? '/ seat / month' : '/ month'}
              </span>
            </p>
            <ul className={cn('mt-8 flex-1 space-y-3 border-t pt-6', plan.highlighted ? 'border-background/15' : 'border-border')}>
              {plan.features.map((f, j) => (
                <li
                  key={f}
                  className="flex items-start gap-2.5 text-sm"
                  style={{ animation: `slide-up 0.6s var(--ease-out-expo) ${300 + j * 60}ms both` }}
                >
                  <Check className={cn('mt-0.5 h-4 w-4 shrink-0', plan.highlighted ? 'text-signal' : 'text-foreground')} />
                  <span>{f}</span>
                </li>
              ))}
            </ul>
            <Link href="/register" className="mt-8" tabIndex={-1}>
              <Button
                className="w-full gap-2"
                variant={plan.highlighted ? 'signal' : 'outline'}
                size="lg"
                roll={false}
              >
                Get started
                <ArrowUpRight className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-45" />
              </Button>
            </Link>
          </Tilt>
        </Reveal>
      ))}
    </div>
  );
}

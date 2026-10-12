'use client';

import { useQuery } from '@tanstack/react-query';

import { getPlanCatalog, type CatalogPlan, type PlanCatalog } from '@/lib/api/plans';
import type { PlanCode } from '@/lib/api/types';

// Plan display for the app (report B02). Prices and limits come from the
// server catalog (GET /plans); only presentation copy lives here. The amount a
// switch actually costs comes from POST /billing/preview-change (proration).

const COPY: Record<PlanCode, { tagline: string; tier: number }> = {
  free: { tagline: 'For trying things out and small teams.', tier: 0 },
  pro: { tagline: 'For growing teams that need more room.', tier: 1 },
  business: { tagline: 'Unlimited scale with usage metering.', tier: 2 },
};

export interface PlanView extends CatalogPlan {
  tagline: string;
  /** Ordinal for upgrade/downgrade comparison. */
  tier: number;
}

export function toPlanViews(catalog: PlanCatalog): PlanView[] {
  return catalog.plans.map((p) => ({ ...p, ...(COPY[p.code] ?? { tagline: '', tier: 0 }) }));
}

/** The catalog, cached for 5 minutes (the endpoint's own cache lifetime). */
export function usePlanCatalog() {
  return useQuery({ queryKey: ['plan-catalog'], queryFn: getPlanCatalog, staleTime: 5 * 60_000 });
}

/** Human count, or "Unlimited" for the -1 sentinel. */
export function limitLabel(n: number): string {
  return n < 0 ? 'Unlimited' : n.toLocaleString();
}

/** Billable seats: members other than guests (FR-TEN-010). */
export function billableSeats(members: { role: string }[]): number {
  return members.filter((m) => m.role !== 'GUEST').length;
}

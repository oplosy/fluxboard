import type { PlanCode } from '@/lib/api/types';

// Marketing copy per plan. Prices and limits are NOT here: the pricing table
// reads them from the server catalog (GET /plans, report B02), so the public
// page always shows what the app enforces and Stripe charges.

export interface MarketingCopy {
  tagline: string;
  /** Non-numeric extras shown after the catalog's limits. */
  extras: string[];
  highlighted?: boolean;
}

export const marketingCopy: Record<PlanCode, MarketingCopy> = {
  free: { tagline: 'For trying things out.', extras: ['Community support'] },
  pro: { tagline: 'For growing teams.', extras: ['Email support'], highlighted: true },
  business: { tagline: 'For scale.', extras: ['Priority support', 'Audit log export'] },
};

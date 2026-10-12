import { apiFetch } from './client';
import type { PlanCode } from './types';

// The plan catalog (report B02): served by GET /plans from the plans table. The
// marketing pricing page and the app's billing pages render every price and
// limit from here; nothing is duplicated in the client.

export interface CatalogPlan {
  code: PlanCode;
  name: string;
  /** Minor units per billable (non-guest) seat per interval. */
  seat_price: number;
  /** -1 = unlimited. */
  max_members: number;
  max_projects: number;
  max_storage_bytes: number;
  api_rate_per_min: number;
  audit_retention_days: number;
  /** Storage and API calls are measured (usage is not charged yet). */
  metered: boolean;
}

export interface PlanCatalog {
  version: string;
  currency: string;
  interval: 'month';
  pricing: 'per_seat';
  plans: CatalogPlan[];
}

export function getPlanCatalog(): Promise<PlanCatalog> {
  return apiFetch<PlanCatalog>('/plans', { skipAuthRetry: true });
}

'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';
import { useOrg } from '@/lib/org/context';
import { useOrgMembers } from '@/lib/org/use-members';
import {
  applyChange,
  cancelSubscription,
  getBillingSummary,
  previewChange,
  startCheckout,
} from '@/lib/api/billing';
import { billableSeats, limitLabel, toPlanViews, usePlanCatalog, type PlanView } from '@/lib/billing/plans';
import { formatMoney, formatBytes } from '@/lib/billing/format';
import type { PlanCode } from '@/lib/api/types';

export default function PlansPage() {
  const { orgId } = useOrg();
  const { members } = useOrgMembers();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const summary = useQuery({
    queryKey: ['billing-summary', orgId],
    queryFn: () => getBillingSummary(orgId),
  });

  const catalog = usePlanCatalog();

  // Target plan for the proration confirm dialog (paid→paid switch only).
  const [switchTo, setSwitchTo] = useState<PlanView | null>(null);

  // Billed seats = non-guest members; the server counts them for checkout too.
  const seats = Math.max(1, billableSeats(members));

  const checkout = useMutation({
    mutationFn: (plan: PlanCode) => startCheckout(orgId, { plan }),
    onSuccess: (url) => {
      window.location.href = url;
    },
    onError: () => toast({ title: 'Couldn’t start checkout', variant: 'error' }),
  });

  const downgradeFree = useMutation({
    mutationFn: () => cancelSubscription(orgId, true),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['billing-summary', orgId] });
      toast({ title: 'Downgrade scheduled for period end', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t schedule the downgrade', variant: 'error' }),
  });

  if (summary.isLoading || catalog.isLoading) {
    return <p className="text-sm text-muted-foreground">Loading plans…</p>;
  }
  if (summary.isError || !summary.data || catalog.isError || !catalog.data) {
    return <p className="text-sm text-destructive">Couldn’t load the plans.</p>;
  }

  const s = summary.data;
  const plans = toPlanViews(catalog.data);
  const currency = catalog.data.currency;
  const current = plans.find((p) => p.code === s.plan);
  const currentTier = current?.tier ?? 0;

  return (
    <div className="space-y-6">
      <p className="text-sm text-muted-foreground">
        You’re on the <span className="font-medium text-foreground">{current?.name ?? s.plan}</span>{' '}
        plan. Paid plans bill per seat — every member except guests ({seats} now) — and the
        seat count follows your members automatically. Switching an active subscription shows
        the exact prorated amount before you confirm.
      </p>

      <div className="grid gap-4 md:grid-cols-3">
        {plans.map((p) => {
          const isCurrent = p.code === s.plan;
          const isUpgrade = p.tier > currentTier;

          return (
            <div
              key={p.code}
              className={`flex flex-col rounded-lg border p-5 ${
                isCurrent ? 'border-primary ring-1 ring-primary' : ''
              }`}
            >
              <div className="mb-3">
                <h3 className="text-lg font-semibold">{p.name}</h3>
                <p className="mt-1 text-sm text-muted-foreground">{p.tagline}</p>
                <p className="mt-3 text-sm">
                  <span className="text-xl font-semibold">{formatMoney(p.seat_price, currency)}</span>
                  <span className="text-muted-foreground"> / seat / month</span>
                  {p.seat_price > 0 ? (
                    <span className="block text-xs text-muted-foreground">
                      {seats} {seats === 1 ? 'seat' : 'seats'} → {formatMoney(p.seat_price * seats, currency)} / month
                    </span>
                  ) : null}
                </p>
              </div>
              <ul className="mb-5 flex-1 space-y-2 text-sm">
                <Feat>{limitLabel(p.max_members)} members</Feat>
                <Feat>{limitLabel(p.max_projects)} projects</Feat>
                <Feat>
                  {limitLabel(p.max_agents)} {p.max_agents === 1 ? 'agent' : 'agents'}
                </Feat>
                <Feat>{formatBytes(p.max_storage_bytes)} storage</Feat>
                <Feat>{p.api_rate_per_min}/min API rate</Feat>
                <Feat>{p.audit_retention_days}-day audit retention</Feat>
                {p.metered ? <Feat>Storage and API usage metered</Feat> : null}
              </ul>

              {isCurrent ? (
                <Button variant="secondary" disabled>
                  Current plan
                </Button>
              ) : p.code === 'free' ? (
                <Button
                  variant="outline"
                  onClick={() => downgradeFree.mutate()}
                  disabled={downgradeFree.isPending || !s.has_subscription}
                >
                  {s.has_subscription ? 'Downgrade to Free' : 'Free'}
                </Button>
              ) : !s.has_subscription ? (
                <Button
                  onClick={() => checkout.mutate(p.code)}
                  disabled={checkout.isPending}
                >
                  {checkout.isPending ? 'Redirecting…' : `Upgrade to ${p.name}`}
                </Button>
              ) : (
                <Button
                  variant={isUpgrade ? 'default' : 'outline'}
                  onClick={() => setSwitchTo(p)}
                >
                  {isUpgrade ? `Upgrade to ${p.name}` : `Switch to ${p.name}`}
                </Button>
              )}
            </div>
          );
        })}
      </div>

      {switchTo ? (
        <ProrationDialog
          orgId={orgId}
          plan={switchTo}
          onClose={() => setSwitchTo(null)}
          onApplied={() => {
            queryClient.invalidateQueries({ queryKey: ['billing-summary', orgId] });
            setSwitchTo(null);
          }}
        />
      ) : null}
    </div>
  );
}

function Feat({ children }: { children: React.ReactNode }) {
  return (
    <li className="flex items-start gap-2">
      <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
      <span>{children}</span>
    </li>
  );
}

function ProrationDialog({
  orgId,
  plan,
  onClose,
  onApplied,
}: {
  orgId: string;
  plan: PlanView;
  onClose: () => void;
  onApplied: () => void;
}) {
  const { toast } = useToast();

  const preview = useQuery({
    queryKey: ['billing-preview', orgId, plan.code],
    queryFn: () => previewChange(orgId, plan.code),
    staleTime: 0,
  });

  const apply = useMutation({
    mutationFn: () => applyChange(orgId, plan.code),
    onSuccess: () => {
      toast({ title: `Switched to ${plan.name}`, variant: 'success' });
      onApplied();
    },
    onError: () => toast({ title: 'Couldn’t change the plan', variant: 'error' }),
  });

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-lg border bg-card p-6 shadow-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold">Switch to {plan.name}</h2>
        <div className="mt-4 rounded-md border bg-secondary/30 p-4 text-sm">
          {preview.isLoading ? (
            <p className="text-muted-foreground">Calculating proration…</p>
          ) : preview.isError || !preview.data ? (
            <p className="text-destructive">Couldn’t calculate the prorated amount.</p>
          ) : (
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Due now (prorated)</span>
              <span className="text-lg font-semibold">
                {formatMoney(preview.data.amount, preview.data.currency)}
              </span>
            </div>
          )}
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          Charged to your existing payment method. Your entitlements change immediately.
        </p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose} disabled={apply.isPending}>
            Cancel
          </Button>
          <Button
            onClick={() => apply.mutate()}
            disabled={apply.isPending || preview.isLoading || preview.isError}
          >
            {apply.isPending ? 'Applying…' : 'Confirm change'}
          </Button>
        </div>
      </div>
    </div>
  );
}

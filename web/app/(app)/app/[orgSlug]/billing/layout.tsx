'use client';

import { usePathname } from 'next/navigation';
import type { ReactNode } from 'react';

import { useOrg } from '@/lib/org/context';
import { SectionShell } from '@/components/org/section-shell';

// Billing shell (docs/02 §9). The whole area is ADMIN+ server-side (read:billing);
// a non-admin sees a notice instead of the tabs (mirrors the settings gate).
export default function BillingLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { slug, isAdmin } = useOrg();
  const base = `/app/${slug}/billing`;

  const nav = [
    { href: base, label: 'Overview', exact: true },
    { href: `${base}/plans`, label: 'Plans' },
    { href: `${base}/usage`, label: 'Usage' },
    { href: `${base}/invoices`, label: 'Invoices' },
  ];

  return (
    <SectionShell
      kicker="Workspace / billing"
      title="Billing"
      tabs={nav.map((n) => ({
        href: n.href,
        label: n.label,
        active: n.exact ? pathname === n.href : pathname.startsWith(n.href),
      }))}
      locked={isAdmin ? undefined : 'Only organization owners and admins can view billing.'}
    >
      {children}
    </SectionShell>
  );
}

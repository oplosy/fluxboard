'use client';

import { usePathname } from 'next/navigation';
import type { ReactNode } from 'react';

import { useOrg } from '@/lib/org/context';
import { SectionShell } from '@/components/org/section-shell';

// Org-settings shell (docs/02 §7). The whole area is ADMIN+ (OWNER-only controls
// are further gated inside General/Danger). A non-admin sees a read-only notice
// instead of the tabs — mirrors the project-settings gate.
export default function SettingsLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { slug, isAdmin } = useOrg();
  const base = `/app/${slug}/settings`;

  const nav = [
    { href: base, label: 'General', exact: true },
    { href: `${base}/members`, label: 'Members' },
    { href: `${base}/labels`, label: 'Labels' },
    { href: `${base}/automations`, label: 'Automations' },
    { href: `${base}/api-keys`, label: 'API keys' },
    { href: `${base}/audit-log`, label: 'Audit log' },
    { href: `${base}/danger`, label: 'Danger zone' },
  ];

  return (
    <SectionShell
      kicker="Workspace / settings"
      title="Organization settings"
      tabs={nav.map((n) => ({
        href: n.href,
        label: n.label,
        active: n.exact ? pathname === n.href : pathname.startsWith(n.href),
      }))}
      locked={isAdmin ? undefined : 'Only organization owners and admins can change organization settings.'}
    >
      {children}
    </SectionShell>
  );
}

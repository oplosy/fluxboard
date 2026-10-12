import type { ReactNode } from 'react';

import { OrgProvider } from '@/lib/org/context';
import { OrgShell } from '@/components/org/shell';

// Org shell (Phase 7 §4). Mounts under the (app) group's RequireVerified guard.
// The static `new-organization/` sibling wins over this dynamic segment, so
// `/app/new-organization` never resolves here.
export default async function OrgLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ orgSlug: string }>;
}) {
  const { orgSlug } = await params;
  return (
    <OrgProvider slug={orgSlug}>
      <OrgShell>{children}</OrgShell>
    </OrgProvider>
  );
}

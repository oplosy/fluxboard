import type { Metadata } from 'next';
import { Kanban, Users, CreditCard, Activity, ShieldCheck, Search, Bell, BarChart3 } from 'lucide-react';

import { PageIntro } from '@/components/marketing/page-intro';
import { FeatureRows } from '@/components/marketing/feature-rows';
import { DotMatrixBand } from '@/components/marketing/dot-matrix-band';

export const metadata: Metadata = { title: 'Features' };

const groups = [
  {
    icon: Kanban,
    title: 'Boards & tasks',
    body: 'Kanban boards with drag-drop ordering, WIP limits, subtasks, labels, comments with @mentions, and attachments. A list view and full-text search keep large projects navigable.',
    tags: ['LexoRank', 'WIP limits', 'Subtasks', '@mentions'],
  },
  {
    icon: Users,
    title: 'Teams & permissions',
    body: 'Organizations with role-based access (Owner, Admin, Member, Guest), project-level membership, and email invitations. Every tenant is isolated at the database level.',
    tags: ['RBAC', 'Invitations', 'RLS'],
  },
  {
    icon: CreditCard,
    title: 'Usage-based billing',
    body: 'Stripe Checkout and Billing Portal, metered seats/storage/API calls, plan enforcement, proration previews, and invoice history.',
    tags: ['Stripe', 'Proration', 'Metering'],
  },
  {
    icon: Activity,
    title: 'Realtime',
    body: 'Server-sent events stream board and task changes to every connected client with surgical cache updates — no manual refresh.',
    tags: ['SSE', 'Last-Event-ID', 'Backoff'],
  },
  {
    icon: Bell,
    title: 'Notifications',
    body: 'An in-app notification center with unread tracking plus per-channel preferences.',
    tags: ['Unread count', 'Preferences'],
  },
  {
    icon: Search,
    title: 'Search & trash',
    body: 'Full-text task search with filters, a cross-project my-tasks view, and a trash with restore/purge.',
    tags: ['Full-text', 'Restore', 'Purge'],
  },
  {
    icon: BarChart3,
    title: 'Analytics',
    body: 'Project analytics — completed-per-week, cumulative flow, cycle time, and per-assignee breakdowns — from nightly rollups.',
    tags: ['Throughput', 'Cumulative flow', 'Cycle time'],
  },
  {
    icon: ShieldCheck,
    title: 'Security & audit',
    body: 'Two-factor authentication, session management, refresh-token rotation, and an org-scoped audit log with CSV export.',
    tags: ['TOTP', 'Sessions', 'Audit CSV'],
  },
];

export default function FeaturesPage() {
  return (
    <>
      <PageIntro
        kicker="Features"
        title="Everything a team needs. Nothing bolted on."
        lede="A complete project-management platform with billing baked in — eight systems that share one data model and one permission model."
      />
      <DotMatrixBand words={['BOARDS', 'TEAMS', 'BILLING', 'LIVE']} />
      <FeatureRows groups={groups.map(({ icon: Icon, ...g }) => ({ ...g, icon: <Icon className="h-5 w-5" strokeWidth={1.6} /> }))} />
    </>
  );
}

import type { Metadata } from 'next';

import { PageIntro } from '@/components/marketing/page-intro';
import { ChangelogTimeline, type ChangelogEntry } from '@/components/marketing/changelog-timeline';

export const metadata: Metadata = { title: 'Changelog' };

// Simple content array (no MDX toolchain needed for a flat changelog).
const entries: ChangelogEntry[] = [
  {
    date: '2026-07-08',
    version: 'Admin & observability',
    changes: [
      'Platform admin panel with tenant management and impersonation.',
      'Org-scoped API keys with read/write scopes.',
      'Prometheus metrics and Grafana dashboards.',
      'OpenAPI 3.1 spec served for client generation.',
    ],
  },
  {
    date: '2026-06-20',
    version: 'Realtime & jobs',
    changes: [
      'Server-sent events for live board updates.',
      'Notification center with per-channel preferences.',
      'Background jobs for email, usage aggregation, and webhook retries.',
    ],
  },
  {
    date: '2026-06-01',
    version: 'Billing',
    changes: [
      'Stripe Checkout, subscriptions, and Billing Portal.',
      'Usage metering for seats, storage, and API calls.',
      'Plan-limit enforcement with upgrade prompts.',
    ],
  },
];

export default function ChangelogPage() {
  return (
    <>
      <PageIntro kicker="Changelog" title="What shipped, and when." lede="Product updates, newest first." />
      <ChangelogTimeline entries={entries} />
    </>
  );
}

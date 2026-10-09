'use client';

import { usePathname } from 'next/navigation';
import { Archive } from 'lucide-react';

import type { Project } from '@/lib/api/types';
import { useOrg } from '@/lib/org/context';
import { SlidingTabs } from '@/components/motion/sliding-tabs';

// Shared header for the project board / list / settings pages: title, key,
// archived badge and the tab nav.
export function ProjectHeader({ project }: { project: Project }) {
  const { slug, isAdmin } = useOrg();
  const pathname = usePathname();
  const base = `/app/${slug}/projects/${project.key}`;

  const tabs = [
    { href: base, label: 'Board', exact: true },
    { href: `${base}/list`, label: 'List', exact: false },
    { href: `${base}/timeline`, label: 'Timeline', exact: false },
    { href: `${base}/calendar`, label: 'Calendar', exact: false },
    { href: `${base}/sprints`, label: 'Sprints', exact: false },
    { href: `${base}/analytics`, label: 'Analytics', exact: false },
    ...(isAdmin ? [{ href: `${base}/settings`, label: 'Settings', exact: false }] : []),
  ];

  // The board tab must stay active while a task modal is open over it.
  const onBoard = pathname === base || /\/tasks\/\d+$/.test(pathname);

  return (
    <div className="mb-6">
      <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
        <span
          className="mb-2 h-3 w-3 shrink-0 rounded-sm"
          style={{ backgroundColor: project.color || 'hsl(var(--muted-foreground))' }}
          aria-hidden
        />
        <h1 className="animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] text-3xl font-semibold tracking-[-0.045em] md:text-4xl">
          {project.name}
        </h1>
        <span className="mb-1.5 rounded-sm border border-border px-1.5 py-0.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
          {project.key}
        </span>
        {project.archived_at ? (
          <span className="mb-1.5 inline-flex items-center gap-1 rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
            <Archive className="h-3 w-3" /> Archived
          </span>
        ) : null}
      </div>
      <div className="mt-5 border-b border-border pb-1">
        <SlidingTabs
          tabs={tabs.map((t) => ({
            href: t.href,
            label: t.label,
            active: t.exact ? onBoard : pathname.startsWith(t.href),
          }))}
        />
      </div>
    </div>
  );
}

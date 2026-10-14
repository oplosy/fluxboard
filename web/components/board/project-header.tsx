'use client';

import { usePathname } from 'next/navigation';
import { Archive } from 'lucide-react';

import type { Project } from '@/lib/api/types';
import { useOrg } from '@/lib/org/context';
import { SlidingTabs } from '@/components/motion/sliding-tabs';

// Shared header for every project tab: title, key, archived badge and the tab
// nav. Its box is fixed (no entrance animation, constant heights) so switching
// tabs never moves it.
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
    <header className="mb-6">
      <div className="flex min-h-10 flex-wrap items-center gap-x-3 gap-y-1">
        <span
          className="h-3.5 w-3.5 shrink-0 rounded-sm"
          style={{ backgroundColor: project.color || 'hsl(var(--muted-foreground))' }}
          aria-hidden
        />
        <h1 className="min-w-0 truncate text-2xl font-semibold tracking-[-0.04em] md:text-3xl">{project.name}</h1>
        <span className="rounded-sm border border-border px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
          {project.key}
        </span>
        {project.archived_at ? (
          <span className="inline-flex items-center gap-1 rounded-sm bg-secondary px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
            <Archive className="h-3 w-3" aria-hidden /> Archived
          </span>
        ) : null}
      </div>
      <div className="mt-4 overflow-x-auto border-b border-border pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        <SlidingTabs
          tabs={tabs.map((t) => ({
            href: t.href,
            label: t.label,
            active: t.exact ? onBoard : pathname.startsWith(t.href),
          }))}
        />
      </div>
    </header>
  );
}
